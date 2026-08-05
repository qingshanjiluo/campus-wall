package repository

import (
	"math"
	"strconv"
	"strings"

	"kun-galgame-api/internal/galgame/model"

	"gorm.io/gorm"
)

// bayesianPriorC is the confidence prior ("virtual votes") for the
// galgame rating Bayesian average: score = (C·m + Σoverall) / (C + n).
// Higher C pulls low-vote-count games harder toward the global mean.
// 10 is a sane default for kungal's modest rating volume — tune here.
const bayesianPriorC = 10.0

// ratingAggJoin pre-aggregates galgame_rating to one row per galgame
// (Σoverall, n). LEFT JOIN so unrated galgames still appear (rt.rsum /
// rt.rcnt NULL). Alias `rt` to avoid clashing with the `gr` galgame_
// resource alias used in the resource-filter branch.
const ratingAggJoin = "LEFT JOIN (SELECT galgame_id, SUM(overall) AS rsum, " +
	"COUNT(*) AS rcnt FROM galgame_rating GROUP BY galgame_id) rt " +
	"ON rt.galgame_id = g.id"

// GalgameListRepository owns the paginated list query with resource-filter
// support (type/language/platform + include/exclude provider sets).
type GalgameListRepository struct {
	db *gorm.DB
}

func NewGalgameListRepository(db *gorm.DB) *GalgameListRepository {
	return &GalgameListRepository{db: db}
}

func (r *GalgameListRepository) DB() *gorm.DB { return r.db }

// allProviders is the closed set of provider names used by the exclude-only filter.
var allProviders = []string{
	"baidu", "aliyun", "quark", "pan123", "tianyiyun",
	"caiyun", "xunlei", "uc", "lanzou", "other",
}

// ListIDs returns galgame IDs matching the given filter, paginated and sorted.
// If hasResourceFilter(f) is true, a JOIN against galgame_resource applies the
// resource-derived filters (and inherently keeps only galgames with a resource).
// Otherwise a simple galgame-only scan — which STILL hides galgames that have no
// download resource via an EXISTS sub-select, unless f.ShowNoResource is set
// (the "显示没有下载资源的 Galgame" toggle, default off).
func (r *GalgameListRepository) ListIDs(f model.GalgameListFilter) (ids []int, total int64) {
	sortCol := "g.resource_update_time"
	// view_1d has no column — it's a live sum of today's view bucket (COALESCE→0
	// when none). Functionally dependent on the PK g.id, so it needs no GROUP BY
	// entry (see groupBy below).
	viewOneDay := "COALESCE((SELECT SUM(d.count) FROM galgame_view_daily d " +
		"WHERE d.entity_id = g.id AND d.day = CURRENT_DATE), 0)"
	switch f.SortField {
	case "time":
		sortCol = "g.resource_update_time"
	case "created":
		sortCol = "g.created"
	case "view":
		sortCol = "g.view"
	case "view_1d":
		sortCol = viewOneDay
	case "view_7d":
		sortCol = "g.view_7d"
	case "view_30d":
		sortCol = "g.view_30d"
	case "release_date":
		sortCol = "g.release_date"
	}
	isSubquerySort := f.SortField == "view_1d"

	ratingSort := f.SortField == "rating"
	ratingFilter := f.MinRatingCount > 0 || f.MinRating > 0

	// Bayesian score expression (computed once: pulls the live global mean
	// m). Built only when a rating sort/filter is active so the AVG query
	// isn't paid otherwise.
	var bayes string
	if ratingSort || ratingFilter {
		bayes = r.bayesianExpr()
	}

	// ORDER BY clause.
	//   rating       → rated first (unrated rt.rcnt IS NULL sinks), then
	//                  the Bayesian score by direction.
	//   release_date → the only other nullable sort col; pin NULLS LAST so
	//                  unknown-date rows don't crowd a "newest first" view.
	orderClause := sortCol + " " + f.SortOrder
	switch {
	case ratingSort:
		orderClause = "(rt.rcnt IS NULL), " + bayes + " " + f.SortOrder
	case sortCol == "g.release_date":
		orderClause += " NULLS LAST"
	}

	type idRow struct {
		ID int `gorm:"column:id"`
	}

	if !hasResourceFilter(f) {
		build := func() *gorm.DB {
			q := r.db.Table("galgame g")
			// nil = no restriction (the global /galgame list). A non-nil set
			// restricts to it via `= ANY(array)`: ONE bound param regardless of
			// size — a mega-tag's ~32k member ids would otherwise expand into a
			// 32k-placeholder IN list — and an empty array naturally matches
			// nothing, so an entity with zero local members yields no rows (never
			// the whole catalogue).
			if f.RestrictIDs != nil {
				q = q.Where("g.id = ANY(?::int[])", intArrayLit(f.RestrictIDs))
			}
			if ratingSort || ratingFilter {
				q = q.Joins(ratingAggJoin)
			}
			q = applyReleaseFilter(q, f)
			q = applyGameTypeFilter(q, f)
			if ratingFilter {
				q = applyRatingFilter(q, f, bayes)
			}
			// Hide galgames with no download resource unless the caller opted in
			// (ShowNoResource). A semi-join EXISTS keeps the plain scan's
			// pagination/order — cheaper than the JOIN+GROUP BY branch below.
			if !f.ShowNoResource {
				q = q.Where("EXISTS (SELECT 1 FROM galgame_resource gr WHERE gr.galgame_id = g.id)")
			}
			return q
		}
		build().Select("COUNT(*)").Scan(&total)
		var rows []idRow
		build().
			Select("g.id").
			Order(orderClause).
			Offset((f.Page - 1) * f.Limit).Limit(f.Limit).
			Scan(&rows)
		ids = make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
		}
		return
	}

	// Join with galgame_resource and apply filters. The rating filter lives
	// in `inner` (it decides which ids qualify); the rating SORT needs the
	// join on the outer query too (to ORDER by it).
	inner := r.db.Table("galgame g").
		Select("DISTINCT g.id").
		Joins("JOIN galgame_resource gr ON gr.galgame_id = g.id")
	// = ANY(array) — one param, empty matches nothing (see the branch above).
	if f.RestrictIDs != nil {
		inner = inner.Where("g.id = ANY(?::int[])", intArrayLit(f.RestrictIDs))
	}
	if ratingFilter {
		inner = inner.Joins(ratingAggJoin)
	}

	inner = applyReleaseFilter(inner, f)
	inner = applyGameTypeFilter(inner, f)
	if f.Type != "" && f.Type != "all" {
		inner = inner.Where("gr.type = ?", f.Type)
	}
	if f.Language != "" && f.Language != "all" {
		inner = inner.Where("gr.language = ?", f.Language)
	}
	if f.Platform != "" && f.Platform != "all" {
		inner = inner.Where("gr.platform = ?", f.Platform)
	}
	if len(f.IncludeProviders) > 0 {
		inner = inner.Where("gr.provider && ?", providerArrayLit(f.IncludeProviders))
	}
	if len(f.ExcludeOnlyProviders) > 0 {
		allowed := providersExcluding(f.ExcludeOnlyProviders)
		if len(allowed) > 0 {
			inner = inner.Where("gr.provider && ?", providerArrayLit(allowed))
		}
	}
	if ratingFilter {
		inner = applyRatingFilter(inner, f, bayes)
	}

	r.db.Table("(?) AS sub", inner).Select("COUNT(*)").Scan(&total)

	main := r.db.Table("galgame g").
		Select("g.id").
		Joins("JOIN galgame_resource gr ON gr.galgame_id = g.id")
	groupBy := "g.id, " + sortCol
	if isSubquerySort {
		groupBy = "g.id"
	}
	if ratingSort {
		main = main.Joins(ratingAggJoin)
		groupBy = "g.id, rt.rsum, rt.rcnt"
	}

	var rows []idRow
	main.
		Where("gr.galgame_id IN (?)", inner).
		Group(groupBy).
		Order(orderClause).
		Offset((f.Page - 1) * f.Limit).Limit(f.Limit).
		Scan(&rows)

	ids = make([]int, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return
}

// bayesianExpr builds the SQL fragment for the galgame Bayesian rating
// average using the LIVE global mean m (cheap: AVG over the small
// galgame_rating table) and the const prior C. m and C are server-owned
// numbers (never user input), so formatting them straight into the
// fragment is injection-safe and avoids pgx param-type inference issues
// in ORDER BY / arithmetic contexts.
//
// References rt.rsum / rt.rcnt from ratingAggJoin; COALESCE handles the
// unrated (LEFT JOIN NULL) case → score = (C·m)/C = m. C>0 guarantees a
// non-zero denominator.
func (r *GalgameListRepository) bayesianExpr() string {
	var m float64
	r.db.Table("galgame_rating").Select("COALESCE(AVG(overall), 0)").Scan(&m)
	c := strconv.FormatFloat(bayesianPriorC, 'f', -1, 64)
	ms := strconv.FormatFloat(m, 'f', 6, 64)
	return "(" + c + " * " + ms + " + COALESCE(rt.rsum, 0)) / (" +
		c + " + COALESCE(rt.rcnt, 0))"
}

// RatingInfo is a galgame's display rating: the Bayesian-smoothed score
// (rounded to 1 dp) and the raw vote count. Count lets the FE distinguish
// "unrated" (omit the badge) from a genuine score.
type RatingInfo struct {
	Score float64
	Count int
}

// BayesianRatings computes the display rating for a page of galgame IDs in
// two cheap queries: the live global mean m (one AVG), then per-id
// Σoverall/n (one grouped scan), combined as (C·m + Σ)/(C + n) — the same
// formula ListIDs sorts/filters by, kept consistent via bayesianPriorC.
// Unrated ids are absent from the map (so the caller omits the badge rather
// than show the prior m as if it were a real score).
func (r *GalgameListRepository) BayesianRatings(ids []int) map[int]RatingInfo {
	out := make(map[int]RatingInfo, len(ids))
	if len(ids) == 0 {
		return out
	}
	var m float64
	r.db.Table("galgame_rating").Select("COALESCE(AVG(overall), 0)").Scan(&m)

	type aggRow struct {
		GalgameID int     `gorm:"column:galgame_id"`
		Rsum      float64 `gorm:"column:rsum"`
		Rcnt      int     `gorm:"column:rcnt"`
	}
	var rows []aggRow
	r.db.Table("galgame_rating").
		Select("galgame_id, SUM(overall) AS rsum, COUNT(*) AS rcnt").
		Where("galgame_id IN ?", ids).
		Group("galgame_id").
		Scan(&rows)

	for _, row := range rows {
		if row.Rcnt == 0 {
			continue
		}
		score := (bayesianPriorC*m + row.Rsum) / (bayesianPriorC + float64(row.Rcnt))
		out[row.GalgameID] = RatingInfo{
			Score: math.Round(score*10) / 10,
			Count: row.Rcnt,
		}
	}
	return out
}

// applyRatingFilter adds the advanced rating WHEREs. minRatingCount is a
// high-confidence gate (only games with enough votes); minRating filters
// on the smoothed Bayesian score so a lone 10/10 doesn't slip through.
func applyRatingFilter(q *gorm.DB, f model.GalgameListFilter, bayes string) *gorm.DB {
	if f.MinRatingCount > 0 {
		q = q.Where("COALESCE(rt.rcnt, 0) >= ?", f.MinRatingCount)
	}
	if f.MinRating > 0 {
		q = q.Where(bayes+" >= ?", f.MinRating)
	}
	return q
}

// applyReleaseFilter adds inclusive release_date bounds when present.
// Bounds are pre-resolved "YYYY-MM-DD" strings (PG casts the literal to
// date). Setting either bound drops NULL release_date rows automatically
// — PG evaluates the comparison to UNKNOWN for NULL, excluding them
// (galgame §17.4). Both empty → no-op, returns the query unchanged.
func applyReleaseFilter(q *gorm.DB, f model.GalgameListFilter) *gorm.DB {
	if f.ReleasedFrom != "" {
		q = q.Where("g.release_date >= ?", f.ReleasedFrom)
	}
	if f.ReleasedTo != "" {
		q = q.Where("g.release_date <= ?", f.ReleasedTo)
	}
	// Discontinuous month set (galgame §17.10), AND-combined with the year
	// range. Non-sargable EXTRACT, but it only rechecks the candidate set
	// the release_date btree range scan already narrowed. NULL release_date
	// → EXTRACT(NULL) → NULL → not IN → dropped, consistent with §17.4.
	if len(f.ReleasedMonths) > 0 {
		q = q.Where("EXTRACT(MONTH FROM g.release_date)::int IN ?", f.ReleasedMonths)
	}
	return q
}

// applyGameTypeFilter narrows to galgames classified as a given WORK type by
// their raters. The type lives in galgame_rating.galgame_type — a JSONB array of
// tags like ["ba_saku","moe"]; a galgame matches a type if ANY of its ratings
// carries that tag (`@> ?`, mirroring the rating-list filter). "uncategorized" =
// no rating tagged the game at all (no non-empty type array). "" / "all" = no-op.
// f.GameType is whitelisted at the DTO (oneof), so the JSON literal is safe.
func applyGameTypeFilter(q *gorm.DB, f model.GalgameListFilter) *gorm.DB {
	switch f.GameType {
	case "", "all":
		return q
	case "uncategorized":
		return q.Where("NOT EXISTS (SELECT 1 FROM galgame_rating grt WHERE grt.galgame_id = g.id AND grt.galgame_type <> '[]'::jsonb)")
	default:
		return q.Where(
			"EXISTS (SELECT 1 FROM galgame_rating grt WHERE grt.galgame_id = g.id AND grt.galgame_type @> ?)",
			"[\""+f.GameType+"\"]",
		)
	}
}

func hasResourceFilter(f model.GalgameListFilter) bool {
	return (f.Type != "" && f.Type != "all") ||
		(f.Language != "" && f.Language != "all") ||
		(f.Platform != "" && f.Platform != "all") ||
		len(f.IncludeProviders) > 0 ||
		len(f.ExcludeOnlyProviders) > 0
}

func providerArrayLit(providers []string) string {
	return "{" + strings.Join(providers, ",") + "}"
}

// intArrayLit renders an int slice as a Postgres array literal ("{1,2,3}", or
// "{}" for empty) for `= ANY(?::int[])` — one bound param instead of an
// N-placeholder IN list, so a mega-tag's tens-of-thousands of member ids stay
// cheap. ids come from the galgame (parsed ints), never user text → injection-safe.
func intArrayLit(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func providersExcluding(excluded []string) []string {
	exSet := map[string]bool{}
	for _, e := range excluded {
		exSet[e] = true
	}
	out := make([]string, 0, len(allProviders))
	for _, p := range allProviders {
		if !exSet[p] {
			out = append(out, p)
		}
	}
	return out
}
