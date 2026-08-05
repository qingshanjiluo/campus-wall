package dto

import "encoding/json"

// MyGalgameInteractions is the current user's liked + favorited galgame ids,
// returned by GET /galgame/interactions/mine to hydrate feed-card like/favorite
// state (the shared feed cache can't carry per-user state).
type MyGalgameInteractions struct {
	Liked     []int `json:"liked"`
	Favorited []int `json:"favorited"`
}

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type GalgameListRequest struct {
	Page                 int    `query:"page" validate:"min=1"`
	Limit                int    `query:"limit" validate:"min=1,max=50"`
	Type                 string `query:"type"`
	Language             string `query:"language"`
	Platform             string `query:"platform"`
	GameType             string `query:"game_type" validate:"omitempty,oneof=all ba_saku plot moe daily uncategorized"`
	SortField            string `query:"sort_field"`
	SortOrder            string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
	IncludeProviders     string `query:"include_providers"`
	ExcludeOnlyProviders string `query:"exclude_only_providers"`
	// Release-date filter, galgame §17 format: "YYYY" or "YYYY-MM" (empty =
	// no bound). Validated + resolved to date boundaries in the service;
	// malformed input → 400.
	ReleasedFrom string `query:"released_from"`
	ReleasedTo   string `query:"released_to"`
	// Discontinuous month set, galgame §17.10: csv of 1–12 (e.g. "3,7").
	// AND-combined with the year range. Malformed → 400.
	ReleasedMonths string `query:"released_months"`
	// Bayesian-rating advanced filters. minRatingCount = high-confidence
	// gate (>= N votes); minRating = Bayesian score >= X (0–10). Sort by
	// rating is driven by sortField=rating.
	MinRatingCount int     `query:"min_rating_count" validate:"omitempty,min=0"`
	MinRating      float64 `query:"min_rating" validate:"omitempty,min=0,max=10"`
	// ShowNoResource controls whether galgames with NO download resources
	// appear. Default false (the "显示没有下载资源的 Galgame" toggle is off) →
	// resource-less galgames are hidden; true → include them.
	ShowNoResource bool `query:"show_no_resource"`
}

// ──────────────────────────────────────────
// Response: list
// ──────────────────────────────────────────

// U2: cover / screenshot rows exposed to the frontend. Mirror the galgame
// wire shape (snake_case) — kungal doesn't rename here because the FE
// stores these in the temp PR store and submits them back unchanged on
// PUT/PR (presence-replace semantics; see frontend Footer). `cdn_url`
// is injected by client.rewriteBanners on every walker pass over a galgame
// response (current + revision/PR snapshots).
type GalgameCover struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	// Kind is the VNDB cover type (main/pkgfront/dig/pkgback/…); empty for user
	// uploads. Sync-managed; the galgame restores it on edit, so echoing it is optional.
	Kind      string `json:"kind,omitempty"`
	CDNURL    string `json:"cdn_url,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

type GalgameScreenshot struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Caption   string `json:"caption"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	CDNURL    string `json:"cdn_url,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

// GalgameListCard matches the existing frontend card used on galgame listings.
// Note: platform/language are denormalised from galgame_resource.
type GalgameListCard struct {
	ID           int         `json:"id"`
	Name         KunLanguage `json:"name"`
	Banner       string      `json:"banner"`
	User         UserBrief   `json:"user"`
	ContentLimit string      `json:"content_limit"`
	View         int         `json:"view"`
	LikeCount    int         `json:"like_count"`
	// Bayesian-smoothed display rating + raw vote count. rating_count 0 =
	// unrated → FE omits the rating badge (rating would otherwise be 0).
	Rating             float64  `json:"rating"`
	RatingCount        int      `json:"rating_count"`
	ResourceUpdateTime string   `json:"resource_update_time"`
	Platform           []string `json:"platform"`
	Language           []string `json:"language"`
	// U1: nil = unknown; cards may sort/filter by release date when set.
	ReleaseDate    *string `json:"release_date"`
	ReleaseDateTBA bool    `json:"release_date_tba"`
	// U2: list cards only need the derived banner. Full covers[] /
	// screenshots[] are detail-only. URL injected by rewriteBanners.
	// banner_image_hash retired in galgame PR5 (K-PR6).
	EffectiveBannerHash      string `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL       string `json:"effective_banner_url,omitempty"`
	EffectiveBannerWidth     int    `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int    `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string `json:"effective_banner_thumbhash,omitempty"`
}

// GalgameListPage is the {galgames, total} envelope for GET /galgame.
type GalgameListPage struct {
	Galgames []GalgameListCard `json:"galgames"`
	Total    int64             `json:"total"`
}

// DraftsPage is the {items, total} envelope for GET /galgame/drafts — the
// unclaimed VNDB drafts (status=2) shown in the detail page's "未发布的游戏"
// modal. Items are enriched GalgameCard[] (IsOnForum=false, Status=2), so the
// shared frontend GalgameCard renders each as a "未在论坛发布" claim card that
// links to the publish wizard — identical to the calendar's status=2 cards.
type DraftsPage struct {
	Items []GalgameCard `json:"items"`
	Total int64         `json:"total"`
}

// ──────────────────────────────────────────
// Response: detail
// ──────────────────────────────────────────

// GalgameDetailOfficial is an official entry on the detail page.
type GalgameDetailOfficial struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Link     string `json:"link"`
	Category string `json:"category"`
	// Roles: this label's part in THIS work (developer / publisher / …), as
	// opposed to Category, which is what kind of organisation it is.
	Roles        []string `json:"roles"`
	Lang         string   `json:"lang"`
	Alias        []string `json:"alias"`
	GalgameCount int      `json:"galgame_count"`
}

// GalgameDetailSeries is one series the game belongs to. Identity only — the
// series page owns the member count.
type GalgameDetailSeries struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GalgameDetailEngine is an engine entry on the detail page.
type GalgameDetailEngine struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Alias        []string `json:"alias"`
	GalgameCount int      `json:"galgame_count"`
}

// GalgameDetailTag is a tag entry on the detail page (with spoiler_level).
type GalgameDetailTag struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	GalgameCount int    `json:"galgame_count"`
	SpoilerLevel int    `json:"spoiler_level"`
}

// GalgameDetailRatingGalgame is the tiny galgame embed inside each rating card.
type GalgameDetailRatingGalgame struct {
	ID           int         `json:"id"`
	ContentLimit string      `json:"content_limit"`
	Name         KunLanguage `json:"name"`
}

// GalgameDetailRating is a rating shown on the galgame detail page.
type GalgameDetailRating struct {
	ID           int                        `json:"id"`
	User         UserBrief                  `json:"user"`
	Recommend    string                     `json:"recommend"`
	Overall      int                        `json:"overall"`
	View         int                        `json:"view"`
	GalgameType  json.RawMessage            `json:"galgame_type"`
	PlayStatus   string                     `json:"play_status"`
	ShortSummary string                     `json:"short_summary"`
	SpoilerLevel string                     `json:"spoiler_level"`
	Art          int                        `json:"art"`
	Story        int                        `json:"story"`
	Music        int                        `json:"music"`
	Character    int                        `json:"character"`
	Route        int                        `json:"route"`
	System       int                        `json:"system"`
	Voice        int                        `json:"voice"`
	ReplayValue  int                        `json:"replay_value"`
	LikeCount    int                        `json:"like_count"`
	IsLiked      bool                       `json:"is_liked"`
	GalgameID    int                        `json:"galgame_id"`
	Created      string                     `json:"created"`
	Updated      string                     `json:"updated"`
	Galgame      GalgameDetailRatingGalgame `json:"galgame"`
}

// GalgameDetail is the full response for GET /galgame/:gid.
type GalgameDetail struct {
	ID                 int         `json:"id"`
	VndbID             string      `json:"vndb_id"`
	User               UserBrief   `json:"user"`
	Name               KunLanguage `json:"name"`
	Banner             string      `json:"banner"`
	Introduction       KunLanguage `json:"introduction"`
	Markdown           KunLanguage `json:"markdown"`
	ContentLimit       string      `json:"content_limit"`
	ResourceUpdateTime string      `json:"resource_update_time"`
	View               int         `json:"view"`
	// IsOnForum is false for a galgame-catalogue galgame the forum has never
	// ingested (no local row). The detail page then shows a 未收录 notice and
	// hides the forum-only view count, keeping the upload/rate/comment CTAs (the
	// recording funnel — those create the local row on first use).
	IsOnForum bool `json:"is_on_forum"`
	// Status = galgame 草稿状态 (0=已发布, 2=VNDB 草稿, 3/4=提交者自己的待审/被拒)。
	// 未收录提示用它判断是否可认领：只有 VNDB 草稿 (status=2) 能被认领成为创建者，
	// 已发布条目 (status=0) 已有创建者、不能再认领。
	Status           int    `json:"status"`
	OriginalLanguage string `json:"original_language"`
	AgeLimit         string `json:"age_limit"`
	// U1 (release_date / release_date_tba): nil = unknown; TBA is
	// independent of the date (a TBA entry may still carry "预计 Y/M").
	ReleaseDate    *string `json:"release_date"`
	ReleaseDateTBA bool    `json:"release_date_tba"`
	// U2: derived effective banner (sort_order=0 cover). URL is injected
	// by client.rewriteBanners so the FE never has to hash → URL on its
	// own. covers/screenshots also receive a `cdn_url` per row from the
	// same walker. banner_image_hash retired in galgame PR5 (K-PR6);
	// covers[sort_order=0] is now the canonical banner source.
	EffectiveBannerHash      string              `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL       string              `json:"effective_banner_url,omitempty"`
	EffectiveBannerWidth     int                 `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int                 `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string              `json:"effective_banner_thumbhash,omitempty"`
	Covers                   []GalgameCover      `json:"covers"`
	Screenshots              []GalgameScreenshot `json:"screenshots"`
	Platform                 []string            `json:"platform"`
	Language                 []string            `json:"language"`
	Type                     []string            `json:"type"`
	Contributor              []UserBrief         `json:"contributor"`
	LikeCount                int                 `json:"like_count"`
	IsLiked                  bool                `json:"is_liked"`
	FavoriteCount            int                 `json:"favorite_count"`
	IsFavorited              bool                `json:"is_favorited"`
	// ResourcePublishBanned: moderators have forbidden publishing download
	// resources under this galgame (copyright / third-party). The FE shows a
	// notice and hides the publish entry.
	ResourcePublishBanned bool                    `json:"resource_publish_banned"`
	Alias                 []string                `json:"alias"`
	Engine                []GalgameDetailEngine   `json:"engine"`
	Official              []GalgameDetailOfficial `json:"official"`
	Series                []GalgameDetailSeries   `json:"series"`
	Tag                   []GalgameDetailTag      `json:"tag"`
	Ratings               []GalgameDetailRating   `json:"ratings"`
	Created               string                  `json:"created"`
	Updated               string                  `json:"updated"`
	// DlsitePurchaseURL / DlsiteCouponURL power the 正版购买 button in the galgame
	// header, assembled server-side (pkg/dlsite) from the catalog's refs.dlsite
	// work number. Empty when the galgame has no DLsite id or the affiliate is
	// unconfigured — the button then does not render.
	DlsitePurchaseURL string `json:"dlsite_purchase_url,omitempty"`
	DlsiteCouponURL   string `json:"dlsite_coupon_url,omitempty"`
}
