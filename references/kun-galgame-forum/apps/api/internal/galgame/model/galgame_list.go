package model

// GalgameListFilter is the parameter bundle for the galgame list repository.
type GalgameListFilter struct {
	Type     string
	Language string
	Platform string
	// GameType filters by the work type that raters assigned in galgame_rating
	// (JSONB array galgame_type, e.g. ["ba_saku","moe"]): a galgame matches if ANY
	// of its ratings carries the tag. "uncategorized" = no rating tagged it;
	// "" / "all" = no filter.
	GameType             string
	SortField            string
	SortOrder            string
	IncludeProviders     []string
	ExcludeOnlyProviders []string
	// Release-date range, already resolved to inclusive "YYYY-MM-DD"
	// boundaries by utils.ParseReleaseLowerBound/UpperBound (empty =
	// no bound on that side). Compared against galgame.release_date;
	// NULL rows drop out once either bound is set (galgame §17.4).
	ReleasedFrom string
	ReleasedTo   string
	// Discontinuous month set (1–12), AND-combined with the year range
	// (galgame §17.10): keep only games whose release month ∈ this set,
	// across all in-range years. Empty = no month filter.
	ReleasedMonths []int
	// Bayesian-rating advanced filters (Design A — computed live from a
	// galgame_rating aggregation join; no denormalized column). The
	// smoothing constants (prior C, global mean m) live inside the repo.
	//   MinRatingCount — keep galgames with at least this many ratings
	//   MinRating      — keep galgames whose Bayesian score >= this (0–10)
	// Zero = filter inactive. Rating SORT is driven by SortField=="rating".
	MinRatingCount int
	MinRating      float64
	// ShowNoResource: when false (default — the "显示没有下载资源的 Galgame"
	// toggle is off), the list is restricted to galgames that have at least
	// one download resource (via the galgame_resource JOIN); when true,
	// resource-less galgames are included too.
	ShowNoResource bool
	// RestrictIDs, when non-empty, scopes the whole list to this id set
	// (g.id IN (…)). Used by the galgame-entity detail pages (tag/official/engine):
	// the galgame supplies the entity's member galgame ids, then the SAME local
	// filter/sort/paginate runs over them. Empty = the global /galgame list.
	RestrictIDs []int
	Page        int
	Limit       int
}

// GalgameResourceMeta holds a platform/language tuple from galgame_resource,
// used when aggregating per-galgame platform/language sets.
type GalgameResourceMeta struct {
	GalgameID int    `gorm:"column:galgame_id"`
	Platform  string `gorm:"column:platform"`
	Language  string `gorm:"column:language"`
}
