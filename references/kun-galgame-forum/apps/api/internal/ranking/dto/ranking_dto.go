package dto

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// SortField uses the FE's short names (the established API surface); the
// repo maps them to the actual galgame columns. `rating` is special-cased
// to a Bayesian average (see FindGalgameLocal). Previously the oneof
// listed raw column names (like_count …) the FE never sends, so every
// non-view galgame sort 400'd — aligned to the short names here.
type GalgameRankingRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=50"`
	SortField string `query:"sort_field" validate:"required,oneof=view like favorite resource rating"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
	// ShowNoResource: false (default) hides galgames with no download resource
	// (the global "显示没有下载资源的 Galgame" toggle); true includes them.
	ShowNoResource bool `query:"show_no_resource"`
}

// SortField uses the FE's short names; the repo (topicSortColumn) maps them to
// the real `*_count` columns. Previously the oneof listed the column names the
// FE never sends, so every non-view topic sort 400'd.
type TopicRankingRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=50"`
	SortField string `query:"sort_field" validate:"required,oneof=view upvote like reply comment favorite"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
}

// SortField: moemoepoint reads kungal_user_state; the rest are per-user COUNT(*)
// over local content tables (see userCountSource). `galgame` is omitted — the
// local galgame table has no creator column post-galgame-migration.
type UserRankingRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=50"`
	SortField string `query:"sort_field" validate:"required,oneof=moemoepoint topic reply_created comment_created galgame_resource"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type UserBrief struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type LocaleName struct {
	EnUS string `json:"en-us"`
	JaJP string `json:"ja-jp"`
	ZhCN string `json:"zh-cn"`
	ZhTW string `json:"zh-tw"`
}

type GalgameRankingItem struct {
	ID   int        `json:"id"`
	Name LocaleName `json:"name"`
	User UserBrief  `json:"user"`
	Banner string   `json:"banner"`
	// float64 so the `rating` sort can carry a Bayesian score (e.g. 8.4);
	// count sorts (view/like…) are whole numbers and marshal without a
	// trailing `.0`.
	Value     float64 `json:"value"`
	SortField string  `json:"sort_field"`
	// U2: derived banner so FE `getEffectiveBanner` can pick `_mini` for
	// covers-only galgames; without these the ranking card falls back to
	// the legacy `banner` URL which is empty for post-PR5 entries.
	// Mirrors HomeGalgame / GalgameListCard.
	EffectiveBannerHash string `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL  string `json:"effective_banner_url,omitempty"`
}

type TopicRankingItem struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	User      UserBrief `json:"user"`
	Value     int       `json:"value"`
	SortField string    `json:"sort_field"`
}

type UserRankingItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Bio    string `json:"bio"`
	Value  int    `json:"value"`
	// SortField must travel back so the FE can render the right
	// sort-icon next to the value. Galgame/Topic already set this; user
	// was missing it (silent FE icon dropout).
	SortField string `json:"sort_field"`
}
