package repository

import (
	"fmt"

	"gorm.io/gorm"
)

type OverviewRepository struct {
	db *gorm.DB
}

func NewOverviewRepository(db *gorm.DB) *OverviewRepository {
	return &OverviewRepository{db: db}
}

// ──────────────────────────────────────────
// Row projections
// ──────────────────────────────────────────

// DailyStat is a single (date, count) row.
type DailyStat struct {
	Date  string `gorm:"column:date"`
	Count int64  `gorm:"column:count"`
}

// ──────────────────────────────────────────
// Reads
// ──────────────────────────────────────────

// CountTable returns the row count of a given table. The table name is
// produced internally by the service and is safe to interpolate.
func (r *OverviewRepository) CountTable(table string) (int64, error) {
	var count int64
	err := r.db.Table(table).Count(&count).Error
	return count, err
}

// DailyCountsSince returns per-day counts for a given table since the given
// `since` timestamp. The table name is produced internally by the service and
// is safe to interpolate.
func (r *OverviewRepository) DailyCountsSince(table string, since any) ([]DailyStat, error) {
	var stats []DailyStat
	err := r.db.Raw(fmt.Sprintf(`
		SELECT date_trunc('day', created)::date::text AS date, COUNT(*) AS count
		FROM %s WHERE created >= ? GROUP BY 1 ORDER BY 1
	`, table), since).Scan(&stats).Error
	return stats, err
}

// CountFeedType returns the number of feed_activity rows of a given type. Since
// the comment areas moved onto the community primitive (charter step 06a) the
// old comment tables are frozen, so the site-wide comment metric is served from
// the materialized feed instead: it counts LIVE (non-tombstoned) comments and is
// near-complete historically (feed rows since the cutover flip; a delete removes
// the row). feedType is a caller-owned constant — never user input.
func (r *OverviewRepository) CountFeedType(feedType string) (int64, error) {
	var count int64
	err := r.db.Table("feed_activity").Where("type = ?", feedType).Count(&count).Error
	return count, err
}

// DailyFeedCountsSince returns per-day feed_activity counts of a given type since
// `since` — the daily-series counterpart of CountFeedType.
func (r *OverviewRepository) DailyFeedCountsSince(feedType string, since any) ([]DailyStat, error) {
	var stats []DailyStat
	err := r.db.Raw(`
		SELECT date_trunc('day', created)::date::text AS date, COUNT(*) AS count
		FROM feed_activity WHERE type = ? AND created >= ? GROUP BY 1 ORDER BY 1
	`, feedType, since).Scan(&stats).Error
	return stats, err
}
