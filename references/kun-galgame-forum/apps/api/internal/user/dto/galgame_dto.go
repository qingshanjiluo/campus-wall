package dto

// KunLanguage is the four-language locale map shared across galgame-related
// responses: { en-us, ja-jp, zh-cn, zh-tw }.
type KunLanguage struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`
}

// UserBrief is the minimal user shape embedded in list responses.
type UserBrief struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// UserGalgameCard is an item in GET /api/user/:userID/galgames.
//
// Wire shape mirrors HomeGalgame/GalgameCard so the FE
// components/user/Galgame.vue can render through the same card
// component as the homepage feed without per-route field guards.
type UserGalgameCard struct {
	ID                 int         `json:"id"`
	Name               KunLanguage `json:"name"`
	Banner             string      `json:"banner"`
	User               UserBrief   `json:"user"`
	ContentLimit       string      `json:"content_limit"`
	View               int         `json:"view"`
	LikeCount          int         `json:"like_count"`
	ResourceUpdateTime string      `json:"resource_update_time"`
	Platform           []string    `json:"platform"`
	Language           []string    `json:"language"`
	// Galgame U1: `released` (free-form string) was replaced by release_date
	// + release_date_tba. FE GalgameCard.release_date is optional, so we
	// pass through the *string (nil → JSON null) and let the FE collapse
	// missing dates to "未知".
	ReleaseDate    *string `json:"release_date"`
	ReleaseDateTBA bool    `json:"release_date_tba"`
	// U2: see HomeGalgame — without these the FE card can't render
	// `_mini` for newly-uploaded (covers-only) galgames.
	EffectiveBannerHash string `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL  string `json:"effective_banner_url,omitempty"`
}

// UserGalgameComment is an item in GET /api/user/:userID/galgame-comments —
// the "评论 / 点赞评论" sub-tabs on /user/:id/galgame/, now sourced from the
// community primitive (charter step 06a). ID is the community post id. Content
// is the raw markdown; ContentHtml the forum-rendered HTML. Deleted marks a
// placeholder row (liked comment since hidden/deleted upstream) so the tab
// renders "已被删除" instead of silently shrinking a keyset page.
type UserGalgameComment struct {
	ID          int64     `json:"id"`
	GalgameID   int       `json:"galgame_id"`
	Content     string    `json:"content"`
	ContentHtml string    `json:"content_html"`
	User        UserBrief `json:"user"`
	Created     string    `json:"created"`
	Deleted     bool      `json:"deleted"`
}

// UserGalgameCommentsRequest is the query for GET /api/user/:userID/galgame-comments.
// Pagination is keyset (opaque `after` cursor) — the old offset `page` is gone
// because the underlying reads are community by-author keyset + local like-table
// keyset.
type UserGalgameCommentsRequest struct {
	Type  string `query:"type" validate:"required"`
	After string `query:"after"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
}
