// Package viewstats maintains per-day view buckets + materialized rolling
// windows (view_7d / view_30d) for galgame / topic / galgame-quiz.
//
// Design (see the discussion that motivated it): the entity keeps its O(1) total
// `view` counter; each view also upserts a `(entity_id, day)` bucket in a small
// per-domain daily table. A daily job (cmd/view-rollup → RunRollup) sums the
// last 7 / 30 days back onto view_7d / view_30d so list pages can ORDER BY a
// window cheaply, then prunes buckets older than keepDays. Calendar-day buckets
// (not an exact rolling window) — the pragmatic, standard forum choice.
package viewstats

import (
	"fmt"

	"gorm.io/gorm"
)

// Per-domain daily-bucket tables. Identical shape (entity_id BIGINT, day DATE,
// count INT, PK(entity_id, day)) so the rollup is generic. These are trusted
// internal constants — NEVER interpolate user input into the SQL below.
const (
	GalgameDaily = "galgame_view_daily"
	TopicDaily   = "topic_view_daily"
	QuizDaily    = "galgame_quiz_view_daily"
)

// keepDays: buckets older than this are pruned. 35 > 30 leaves a small margin so
// the 30-day window is always complete right after a prune.
const keepDays = 35

// pairs maps each daily-bucket table to the entity table whose view_7d/view_30d
// columns it feeds.
var pairs = []struct{ daily, entity string }{
	{GalgameDaily, "galgame"},
	{TopicDaily, "topic"},
	{QuizDaily, "galgame_quiz"},
}

// BumpDaily upserts +1 into today's (entity_id, day) bucket. Best-effort — view
// stats are non-critical, so callers ignore the returned error.
func BumpDaily(db *gorm.DB, table string, entityID int) error {
	return db.Exec(fmt.Sprintf(
		`INSERT INTO %s (entity_id, day, count) VALUES (?, CURRENT_DATE, 1)
		 ON CONFLICT (entity_id, day) DO UPDATE SET count = %s.count + 1`,
		table, table), entityID).Error
}

// RunRollup recomputes view_7d / view_30d on every entity table from its daily
// buckets, then prunes stale buckets. Idempotent — run once per day.
func RunRollup(db *gorm.DB) error {
	for _, p := range pairs {
		if err := rollupOne(db, p.daily, p.entity); err != nil {
			return fmt.Errorf("rollup %s: %w", p.entity, err)
		}
	}
	return nil
}

func rollupOne(db *gorm.DB, daily, entity string) error {
	// Recompute for entities that have a bucket in the last 30 days.
	if err := db.Exec(fmt.Sprintf(`
		WITH agg AS (
			SELECT entity_id,
				COALESCE(SUM(count) FILTER (WHERE day >= CURRENT_DATE - INTERVAL '6 days'), 0)  AS v7,
				COALESCE(SUM(count) FILTER (WHERE day >= CURRENT_DATE - INTERVAL '29 days'), 0) AS v30
			FROM %s
			WHERE day >= CURRENT_DATE - INTERVAL '29 days'
			GROUP BY entity_id
		)
		UPDATE %s e SET view_7d = agg.v7, view_30d = agg.v30
		FROM agg WHERE e.id = agg.entity_id`, daily, entity)).Error; err != nil {
		return err
	}
	// Zero out entities whose window emptied but still carry a nonzero value.
	if err := db.Exec(fmt.Sprintf(`
		UPDATE %s e SET view_7d = 0, view_30d = 0
		WHERE (e.view_7d <> 0 OR e.view_30d <> 0)
		  AND NOT EXISTS (
			SELECT 1 FROM %s d
			WHERE d.entity_id = e.id AND d.day >= CURRENT_DATE - INTERVAL '29 days')`,
		entity, daily)).Error; err != nil {
		return err
	}
	// Prune old buckets.
	return db.Exec(fmt.Sprintf(
		`DELETE FROM %s WHERE day < CURRENT_DATE - INTERVAL '%d days'`, daily, keepDays)).Error
}
