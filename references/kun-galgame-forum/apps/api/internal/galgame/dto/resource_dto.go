package dto

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type ResourceListRequest struct {
	Page  int `query:"page" validate:"min=1"`
	Limit int `query:"limit" validate:"min=1,max=50"`
}

type GalgameResourcesRequest struct {
	GalgameID int `query:"galgame_id" validate:"required,min=1"`
}

// CreateGalgameResourceRequest is the body of POST /galgame/:gid/resource.
type CreateGalgameResourceRequest struct {
	GalgameID int      `json:"galgame_id" validate:"required,min=1"`
	Type      string   `json:"type" validate:"required"`
	Language  string   `json:"language" validate:"required"`
	Platform  string   `json:"platform" validate:"required"`
	Size      string   `json:"size" validate:"required,max=107"`
	Code      string   `json:"code" validate:"max=1007"`
	Password  string   `json:"password" validate:"max=1007"`
	Note      string   `json:"note" validate:"max=10000"`
	Link      []string `json:"link" validate:"required,min=1,max=20,dive,url"`
}

// UpdateGalgameResourceRequest is the body of PUT /galgame/:gid/resource.
type UpdateGalgameResourceRequest struct {
	GalgameResourceID int      `json:"galgame_resource_id" validate:"required,min=1"`
	GalgameID         int      `json:"galgame_id"`
	Type              string   `json:"type" validate:"required"`
	Language          string   `json:"language" validate:"required"`
	Platform          string   `json:"platform" validate:"required"`
	Size              string   `json:"size" validate:"required,max=107"`
	Code              string   `json:"code" validate:"max=1007"`
	Password          string   `json:"password" validate:"max=1007"`
	Note              string   `json:"note" validate:"max=10000"`
	Link              []string `json:"link" validate:"required,min=1,max=20,dive,url"`
}

// DeleteGalgameResourceRequest is the query for DELETE /galgame/:gid/resource.
type DeleteGalgameResourceRequest struct {
	GalgameResourceID int `query:"galgame_resource_id" validate:"required,min=1"`
}

// ToggleResourceLikeRequest is the body of PUT /galgame/:gid/resource/like.
type ToggleResourceLikeRequest struct {
	GalgameResourceID int `json:"galgame_resource_id" validate:"required,min=1"`
}

// ResourceStatusRequest is the body of PUT valid/expired endpoints.
type ResourceStatusRequest struct {
	GalgameResourceID int `json:"galgame_resource_id" validate:"required,min=1"`
}

// ReportExpireResult is the 200 body of PUT /resource/expired. `verdict` is the
// link checker's resource-level result ("alive" / "dead" / "unknown", or "" when
// the checker is unconfigured or the resource has no links). `marked` is whether
// the resource was actually flipped to expired — false ONLY when the link was
// verified still reachable ("alive"). The FE drives its check → mark status UI
// off these so "still alive" reads as a friendly outcome, not a failure.
type ReportExpireResult struct {
	Verdict string `json:"verdict"`
	Marked  bool   `json:"marked"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

// UserBrief is a lightweight user projection used in resource responses.
type UserBrief struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// KunLanguage is a four-language text map.
type KunLanguage struct {
	EnUs string `json:"en-us"`
	JaJp string `json:"ja-jp"`
	ZhCn string `json:"zh-cn"`
	ZhTw string `json:"zh-tw"`
}

// ResourceCard is the shape returned in list views (no links/code/password).
type ResourceCard struct {
	ID        int       `json:"id"`
	View      int       `json:"view"`
	GalgameID int       `json:"galgame_id"`
	User      UserBrief `json:"user"`
	Type      string    `json:"type"`
	Language  string    `json:"language"`
	Platform  string    `json:"platform"`
	Size      string    `json:"size"`
	Status    int       `json:"status"`
	Download  int       `json:"download"`
	LikeCount int       `json:"like_count"`
	IsLiked   bool      `json:"is_liked"`
	// CommentCount is the resource comment counter (migration 065), maintained ±1
	// by the community comment BFF. A tolerated display counter.
	CommentCount int `json:"comment_count"`
	// DlsitePurchaseURL — see the field of the same name on ResourceMeta. Present
	// here because the download modal is also opened from the resource CARD on a
	// galgame detail page, where no galgame object is in scope.
	DlsitePurchaseURL string      `json:"dlsite_purchase_url,omitempty"`
	DlsiteCouponURL   string      `json:"dlsite_coupon_url,omitempty"`
	LinkDomain        string      `json:"link_domain"`
	ProviderNames     []string    `json:"provider_names"`
	Note              string      `json:"note"`
	NoteHtml          string      `json:"note_html"`
	Created           string      `json:"created"`
	Edited            *string     `json:"edited"`
	GalgameName       KunLanguage `json:"galgame_name,omitempty"`
}

// ResourceMeta is the safe (no credentials) view of a single resource —
// returned by the page endpoint GET /galgame-resource/:id. The FE
// type `GalgameResource` mirrors this shape and the dedicated
// LinkDetailModal calls GET /galgame-resource/:id/detail when the user
// actually clicks the download button.
//
// Keeping link/code/password OFF this endpoint matters for two
// reasons: (a) the page endpoint is in the optAuth group so anonymous
// scrapers can fetch it; leaking credentials there bypasses the
// per-download moemoepoint check. (b) the dedicated /detail call is
// what bumps the download counter — if the page already shipped the
// link, the counter never moves.
type ResourceMeta struct {
	ID           int       `json:"id"`
	View         int       `json:"view"`
	GalgameID    int       `json:"galgame_id"`
	User         UserBrief `json:"user"`
	Type         string    `json:"type"`
	Language     string    `json:"language"`
	Platform     string    `json:"platform"`
	Size         string    `json:"size"`
	Status       int       `json:"status"`
	Download     int       `json:"download"`
	LikeCount    int       `json:"like_count"`
	IsLiked      bool      `json:"is_liked"`
	CommentCount int       `json:"comment_count"`
	// DlsitePurchaseURL is the ready-to-use DLsite affiliate purchase link shown
	// in the 补票 prompt, assembled server-side (pkg/dlsite) from the galgame's
	// catalog refs.dlsite work number. It rides the RESOURCE rather than the
	// galgame because both 补票 surfaces (the download modal and the resource
	// detail panel) are resource-scoped — the modal is opened from contexts that
	// hold no galgame object, so putting it here avoids drilling the URL through
	// several component layers.
	//
	// Empty when the galgame has no DLsite id, the affiliate template is
	// unconfigured, or this payload comes from a path that renders no 补票 prompt.
	// The frontend never builds this URL: the affiliate template stays in server
	// config, out of the browser bundle.
	DlsitePurchaseURL string `json:"dlsite_purchase_url,omitempty"`
	// DlsiteCouponURL is the partnership's coupon landing page (a GLOBAL benefit,
	// not per-work). Emitted only together with a purchase link, since the notice
	// shows them as one block. Must already be a shortened URL — see DlsiteConfig.
	DlsiteCouponURL string   `json:"dlsite_coupon_url,omitempty"`
	LinkDomain      string   `json:"link_domain"`
	ProviderNames   []string `json:"provider_names"`
	Note            string   `json:"note"`
	NoteHtml        string   `json:"note_html"`
	Created         string   `json:"created"`
	Edited          *string  `json:"edited"`
}

// ResourceDownloadDetail is returned by GET /galgame-resource/:id/detail.
// Includes download links, code, password — the actual credentials.
// Endpoint also bumps the download counter as a side effect, so call
// it once per user click.
type ResourceDownloadDetail struct {
	ResourceMeta
	Link     []string `json:"link"`
	Code     string   `json:"code"`
	Password string   `json:"password"`
}

// ResourceGalgameSummary is the galgame info shown on resource detail page.
type ResourceGalgameSummary struct {
	ID     int         `json:"id"`
	Name   KunLanguage `json:"name"`
	Banner string      `json:"banner"`
	// U2 banner pair — FE Hero / [id]/index.vue both call
	// `getEffectiveBanner(galgame)` which reads these before falling
	// back to legacy `banner`. Missing them broke the hero on
	// covers-only (post galgame PR5) galgames.
	EffectiveBannerHash      string   `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL       string   `json:"effective_banner_url,omitempty"`
	EffectiveBannerWidth     int      `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int      `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string   `json:"effective_banner_thumbhash,omitempty"`
	ContentLimit             string   `json:"content_limit"`
	View                     int      `json:"view"`
	ResourceUpdateTime       string   `json:"resource_update_time"`
	OriginalLanguage         string   `json:"original_language"`
	AgeLimit                 string   `json:"age_limit"`
	Platform                 []string `json:"platform"`
	Language                 []string `json:"language"`
	Type                     []string `json:"type"`
}

// ResourceDetailPage is the full response for GET /galgame-resource/:id.
type ResourceDetailPage struct {
	Galgame         ResourceGalgameSummary `json:"galgame"`
	Resource        ResourceMeta           `json:"resource"`
	Recommendations []ResourceCard         `json:"recommendations"`
}

// ResourceListPage is the response for GET /galgame-resource.
type ResourceListPage struct {
	Resources []ResourceCard `json:"resources"`
	Total     int64          `json:"total"`
}
