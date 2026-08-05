package repository

import (
	"strconv"

	"gorm.io/gorm"
)

// rankingBayesianPriorC mirrors galgame/repository.bayesianPriorC — the
// confidence prior for the rating Bayesian average. Kept in sync by hand
// (the two modules don't share a package); tune both together.
const rankingBayesianPriorC = 10.0

// galgameSortColumn maps the FE's short sort names to the real galgame
// columns. `rating` is NOT here — it's special-cased to a Bayesian score.
var galgameSortColumn = map[string]string{
	"view":     "view",
	"like":     "like_count",
	"favorite": "favorite_count",
	"resource": "resource_count",
}

// topicSortColumn maps the FE's short topic-ranking sort names to the real
// topic columns. The FE sends `upvote`/`like`/… but the columns are
// `upvote_count`/`like_count`/… — the old validator listed the column names
// the FE never sends, so every non-view topic sort 400'd. Same fix the
// galgame ranking already had.
var topicSortColumn = map[string]string{
	"view":     "view",
	"upvote":   "upvote_count",
	"like":     "like_count",
	"reply":    "reply_count",
	"comment":  "comment_count",
	"favorite": "favorite_count",
}

// userCountSource maps the FE's user-ranking sort names to a LOCAL table whose
// per-user COUNT(*) is the ranking value. `moemoepoint` is handled separately
// (it's a kungal_user_state column, not a count). The `galgame` ("Galgame 数")
// metric is intentionally absent: post-galgame-migration the local `galgame`
// table has no creator column, so it has no local source — ranking by it would
// need galgame data (infra-owned).
var userCountSource = map[string]struct {
	table string
	where string
}{
	"topic":            {table: "topic", where: "status != 1"}, // exclude banned, mirrors topic ranking
	"reply_created":    {table: "topic_reply"},
	"comment_created":  {table: "topic_comment"},
	"galgame_resource": {table: "galgame_resource"},
}

type RankingRepository struct {
	db *gorm.DB
}

func NewRankingRepository(db *gorm.DB) *RankingRepository {
	return &RankingRepository{db: db}
}

// ──────────────────────────────────────────
// Row projections
// ──────────────────────────────────────────

// Value is float64 so the `rating` sort can carry a Bayesian score; count
// sorts produce whole numbers.
type GalgameLocalRow struct {
	ID    int     `gorm:"column:id"`
	Value float64 `gorm:"column:value"`
	// CreatorUserID is the frozen wiki-era submitter (migration 066) — the
	// author chip's only source, since the catalog face carries no submitter.
	// NULL = unknown, which the ranking row renders as no chip.
	CreatorUserID *int `gorm:"column:creator_user_id"`
}

// TopicRankingRow returns a topic ranking row. Identity is hydrated by the
// service layer via userclient.
type TopicRankingRow struct {
	ID     int    `gorm:"column:id"`
	Title  string `gorm:"column:title"`
	UserID int    `gorm:"column:user_id"`
	Value  int    `gorm:"column:value"`
}

// UserRankingRow returns a user ranking row keyed by user_id. Sorting fields
// (e.g. moemoepoint) live in kungal_user_state. Identity (name/avatar/bio) is
// hydrated by the service layer via userclient.
type UserRankingRow struct {
	UserID int `gorm:"column:user_id"`
	Value  int `gorm:"column:value"`
}

// ──────────────────────────────────────────
// Queries
// ──────────────────────────────────────────

// FindGalgameLocal returns (id, sort_value) pairs sorted by the requested
// field. `rating` is a Bayesian average over galgame_rating (rated games
// only — a rating leaderboard shouldn't list unrated titles); the other
// fields map to galgame columns. sortField/sortOrder are validator-
// constrained, so the column interpolation is safe.
// showNoResource=false (default) hides galgames with no download resource via an
// EXISTS semi-join — the global "显示没有下载资源的 Galgame" toggle.
func (r *RankingRepository) FindGalgameLocal(sortField, sortOrder string, page, limit int, showNoResource bool) []GalgameLocalRow {
	var rows []GalgameLocalRow

	if sortField == "rating" {
		var m float64
		r.db.Table("galgame_rating").Select("COALESCE(AVG(overall), 0)").Scan(&m)
		c := strconv.FormatFloat(rankingBayesianPriorC, 'f', -1, 64)
		ms := strconv.FormatFloat(m, 'f', 6, 64)
		bayes := "(" + c + " * " + ms + " + rt.rsum) / (" + c + " + rt.rcnt)"
		q := r.db.Table("galgame g").
			Joins("JOIN (SELECT galgame_id, SUM(overall) AS rsum, COUNT(*) AS rcnt " +
				"FROM galgame_rating GROUP BY galgame_id) rt ON rt.galgame_id = g.id").
			Select("g.id, g.creator_user_id, ROUND((" + bayes + ")::numeric, 2) AS value")
		if !showNoResource {
			q = q.Where("EXISTS (SELECT 1 FROM galgame_resource gr WHERE gr.galgame_id = g.id)")
		}
		q.Order(bayes + " " + sortOrder).
			Offset((page - 1) * limit).
			Limit(limit).
			Scan(&rows)
		return rows
	}

	col := galgameSortColumn[sortField]
	if col == "" {
		col = "view"
	}
	q := r.db.Table("galgame").
		Select("id, creator_user_id, " + col + " AS value")
	if !showNoResource {
		q = q.Where("EXISTS (SELECT 1 FROM galgame_resource gr WHERE gr.galgame_id = galgame.id)")
	}
	q.Order(col + " " + sortOrder).
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&rows)
	return rows
}

// FindTopicRanking returns topic ranking rows. Identity is hydrated at the
// service layer.
//
// isSFW=true filters out is_nsfw=true rows so anonymous / SEO-bot callers
// can't crawl NSFW topics through the ranking page (and so cookie-off
// users get a clean list). is_nsfw is kungal-local data, so the filter
// is correctly applied at the SQL layer here (unlike galgame.content_limit
// which lives only on galgame).
func (r *RankingRepository) FindTopicRanking(sortField, sortOrder string, page, limit int, isSFW bool) []TopicRankingRow {
	var rows []TopicRankingRow
	col := topicSortColumn[sortField]
	if col == "" {
		col = "view"
	}
	q := r.db.Table("topic t").
		Select(`t.id, t.title, t.user_id, t.` + col + ` AS value`).
		Where("t.status != 1")
	if isSFW {
		q = q.Where("t.is_nsfw = false")
	}
	q.Order("t." + col + " " + sortOrder).
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows)
	return rows
}

// FindUserRanking returns user ranking rows ordered by a kungal_user_state
// column (currently only moemoepoint). Identity (name/avatar/bio) is hydrated
// at the service layer via userclient since the user table is no longer the
// source of truth.
func (r *RankingRepository) FindUserRanking(sortField, sortOrder string, page, limit int) []UserRankingRow {
	var rows []UserRankingRow

	// moemoepoint is the only rankable column on kungal_user_state.
	if sortField == "moemoepoint" {
		r.db.Table("kungal_user_state").
			Select("user_id, moemoepoint AS value").
			Order("moemoepoint " + sortOrder).
			Offset((page - 1) * limit).Limit(limit).
			Find(&rows)
		return rows
	}

	// The other metrics are per-user COUNT(*) over a local content table.
	// sortField is validator-constrained, so the map lookup is safe and only
	// known sources reach the SQL. Users with zero of a metric simply don't
	// appear — fine for a leaderboard.
	src, ok := userCountSource[sortField]
	if !ok {
		return rows
	}
	q := r.db.Table(src.table).Select("user_id, COUNT(*) AS value")
	if src.where != "" {
		q = q.Where(src.where)
	}
	q.Group("user_id").
		Order("value " + sortOrder).
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows)
	return rows
}
