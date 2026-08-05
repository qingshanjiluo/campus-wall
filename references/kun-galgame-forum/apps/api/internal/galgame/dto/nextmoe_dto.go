package dto

// This file holds parsing structs for Galgame Service responses.
// These types mirror the wire format produced by the galgame service;
// they are used by services when decoding `json.RawMessage` payloads.
//
// The fields are a superset — consumers ignore what they don't need.

// NextMoeAlias is a named alias entry used by Official/Tag/Galgame.
type NextMoeAlias struct {
	Name string `json:"name"`
}

// NextMoeGalgameItem is the shape returned inside list/detail responses of
// series/official/engine/tag endpoints (a "lite" galgame summary).
//
// Note: galgame returns `view` here too but kungal owns view as a per-site
// stat — see NextMoeGalgameDetailFull comment. We don't parse it; enricher
// reads view from the local stats row.
type NextMoeGalgameItem struct {
	ID                 int     `json:"id"`
	NameEnUs           string  `json:"name_en_us"`
	NameJaJp           string  `json:"name_ja_jp"`
	NameZhCn           string  `json:"name_zh_cn"`
	NameZhTw           string  `json:"name_zh_tw"`
	Banner             string  `json:"banner"`
	ContentLimit       string  `json:"content_limit"`
	ResourceUpdateTime string  `json:"resource_update_time"`
	ReleaseDate        *string `json:"release_date"`
	ReleaseDateTBA     bool    `json:"release_date_tba"`
	// U2: derived effective banner hash on list rows (galgame computes it
	// from covers[sort_order=0]). EffectiveBannerURL is injected by
	// client.rewriteBanners BEFORE we unmarshal — capture it
	// explicitly or Go's unmarshal drops the walker's work.
	// banner_image_hash was retired in galgame PR5 (K-PR6) — no field.
	EffectiveBannerHash      string `json:"effective_banner_hash"`
	EffectiveBannerURL       string `json:"effective_banner_url"`
	EffectiveBannerWidth     int    `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int    `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string `json:"effective_banner_thumbhash,omitempty"`
	UserID                   int    `json:"user_id"`
	// ReleasePrecision marks how to read ReleaseDate (day/month/year/tba/unknown).
	// Only the calendar endpoints (GET /galgame/calendar[/pending|/tba]) emit it;
	// other list/detail galgame responses omit it → "" here. See the calendar §
	// release_precision in docs/galgame_wiki/01-galgame.md.
	ReleasePrecision string `json:"release_precision"`
	// Status = galgame 草稿状态. The calendar now returns status IN (0,2): 0=已发布,
	// 2=未认领的 VNDB 草稿 (claimable). Threaded to the card so the FE can render
	// drafts as "未发布" (claim flow) instead of linking to /galgame/:gid (404 for
	// drafts). 0 elsewhere (entity/list responses are published-only).
	Status int `json:"status"`
}

// U2 cover/screenshot row shapes (snake_case, matches galgame wire). Both
// share the scalar fields; screenshot additionally has Caption.
//
// CDNURL: like EffectiveBannerURL above, client.rewriteBanners injects a
// per-row `cdn_url` (image_hash → CDN URL) into the galgame bytes BEFORE we
// unmarshal — this field MUST be declared or Go's unmarshal silently drops
// the walker's work and the gallery renders no images. (This was the bug:
// the field was missing here, so screenshots/covers reached the FE without a
// cdn_url and the gallery fell back to a /image/<hash> redirect per image.)
type NextMoeGalgameCover struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	// Kind labels the VNDB cover type (main/pkgfront/dig/pkgback/…); "" for user
	// uploads. Declared so it survives unmarshal — the "查看所有封面" modal groups
	// covers by it (without this every cover fell into 其它).
	Kind      string `json:"kind,omitempty"`
	CDNURL    string `json:"cdn_url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

type NextMoeGalgameScreenshot struct {
	ImageHash string `json:"image_hash"`
	SortOrder int    `json:"sort_order"`
	Caption   string `json:"caption"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
	SourceKey string `json:"source_key"`
	CDNURL    string `json:"cdn_url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbhash string `json:"thumbhash,omitempty"`
}

// NextMoeOfficial is a company/publisher/developer entity from the galgame.
//
// `galgame_count` is published by galgame since K-PR (2026-05-22): the
// detail endpoint's Preload now injects the same `COUNT(*) AS cnt`
// subquery the list endpoint uses, so the "会社名 +N" chip on the
// galgame detail page can be filled in a single round-trip. Field
// defaults to 0 (omitempty intentionally not used — 0 is the correct
// answer for an official that exists but currently has zero published
// galgames).
type NextMoeOfficial struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Link     string `json:"link"`
	Category string `json:"category"`
	// Roles is what this label DID on the work it is attached to
	// (developer / publisher / circle / brand), in the registry's own
	// vocabulary. Category answers a different question — what KIND of
	// organisation it is — and the two must not be collapsed: a brand that
	// both developed and published a title carries two roles and one category.
	Roles        []string       `json:"roles"`
	Lang         string         `json:"lang"`
	Alias        []NextMoeAlias `json:"alias"`
	GalgameCount int            `json:"galgame_count"`
}

// NextMoeOfficialRel is the wrapper used when an official is attached to a galgame.
type NextMoeOfficialRel struct {
	Official NextMoeOfficial `json:"official"`
}

// NextMoeEngine is a game engine entity.
type NextMoeEngine struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Link  string `json:"link"`
	Intro string `json:"intro"`
}

// NextMoeEngineRel wraps an engine attached to a galgame.
type NextMoeEngineRel struct {
	Engine NextMoeEngine `json:"engine"`
}

// NextMoeTag is a tag entity with optional aliases.
//
// `galgame_count` is published since K-PR (2026-05-22) — same source
// as NextMoeOfficial.GalgameCount (galgame's detail Preload injects a COUNT
// subquery filtered on galgame.status = 0). Used to render the
// "+N" chip on tag bubbles in the galgame detail page.
type NextMoeTag struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Category     string         `json:"category"`
	Alias        []NextMoeAlias `json:"alias"`
	GalgameCount int            `json:"galgame_count"`
}

// NextMoeTagRel wraps a tag attached to a galgame.
type NextMoeTagRel struct {
	Tag NextMoeTag `json:"tag"`
}

// NextMoeContributor represents a user who contributed to a galgame.
type NextMoeContributor struct {
	UserID int `json:"user_id"`
}

// NextMoeGalgameDetail is the core galgame payload returned by /galgame/:id.
// It contains the fields commonly consumed by the gateway service.
type NextMoeGalgameDetail struct {
	ID       int    `json:"id"`
	NameEnUs string `json:"name_en_us"`
	NameJaJp string `json:"name_ja_jp"`
	NameZhCn string `json:"name_zh_cn"`
	NameZhTw string `json:"name_zh_tw"`
	Banner   string `json:"banner"`
	// U2 derived banner pair (galgame PR5). Forwarded so consumers like the
	// rating detail service can ship them downstream — without these,
	// covers-only galgames render an empty hero on the FE.
	EffectiveBannerHash      string `json:"effective_banner_hash"`
	EffectiveBannerURL       string `json:"effective_banner_url"`
	EffectiveBannerWidth     int    `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int    `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string `json:"effective_banner_thumbhash,omitempty"`
	ContentLimit             string `json:"content_limit"`
	AgeLimit                 string `json:"age_limit"`
	OriginalLanguage         string `json:"original_language"`
	// nil when the galgame isn't part of any series. Used by the rating
	// detail service to attach the minimal series brief for JSON-LD.
	SeriesID     *int                 `json:"series_id"`
	Official     []NextMoeOfficialRel `json:"official"`
	Engine       []NextMoeEngineRel   `json:"engine"`
	Tag          []NextMoeTagRel      `json:"tag"`
	Contributors []NextMoeContributor `json:"contributors"`
}

// NextMoeGalgameDetailResponse is the envelope: {galgame: {...}}.
type NextMoeGalgameDetailResponse struct {
	Galgame NextMoeGalgameDetail `json:"galgame"`
}

// NextMoeUser mirrors the user shape returned by galgame inside the `users` map of
// detail responses.
type NextMoeUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// NextMoeTagWithSpoiler extends NextMoeTag with a spoiler_level annotation used
// only in galgame detail responses.
type NextMoeTagWithSpoiler struct {
	SpoilerLevel int        `json:"spoiler_level"`
	Tag          NextMoeTag `json:"tag"`
}

// NextMoeEngineAlias is a flat alias slice (used in list/detail shape).
type NextMoeEngineAlias []string

// NextMoeGalgameDetailFull is the superset of fields returned by GET /galgame/:id.
// It includes nested alias/official/engine/tag/contributor/intro/user_id.
type NextMoeGalgameDetailFull struct {
	ID           int    `json:"id"`
	VndbID       string `json:"vndb_id"`
	NameEnUs     string `json:"name_en_us"`
	NameJaJp     string `json:"name_ja_jp"`
	NameZhCn     string `json:"name_zh_cn"`
	NameZhTw     string `json:"name_zh_tw"`
	Banner       string `json:"banner"`
	IntroEnUs    string `json:"intro_en_us"`
	IntroJaJp    string `json:"intro_ja_jp"`
	IntroZhCn    string `json:"intro_zh_cn"`
	IntroZhTw    string `json:"intro_zh_tw"`
	ContentLimit string `json:"content_limit"`
	// Galgame also returns `view`, but kungal owns view as a per-site stat
	// (each site has its own user base, so galgame's cross-site view doesn't
	// fit the on-page "this many people viewed on kungal" semantics).
	// Intentionally not parsed — see GetDetail in galgame_service.go which
	// reads view from the local galgame stats row instead.
	ResourceUpdateTime string `json:"resource_update_time"`
	// Galgame upgrade U1: `released` (free-form string) was replaced by a
	// proper date column + a TBA flag. `*string "YYYY-MM-DD"` keeps tz/
	// precision out of revision diffs (wire is plain string, not Time).
	ReleaseDate      *string `json:"release_date"`
	ReleaseDateTBA   bool    `json:"release_date_tba"`
	OriginalLanguage string  `json:"original_language"`
	AgeLimit         string  `json:"age_limit"`
	UserID           int     `json:"user_id"`
	SeriesID         *int    `json:"series_id"`
	Status           int     `json:"status"`
	// U2: cover candidate set + screenshot gallery + derived effective
	// banner. galgame PR5 (K-PR6) dropped the legacy banner_image_hash
	// top-level field — banner is now expressed solely through
	// covers[sort_order=0] and its derived effective_banner_hash.
	EffectiveBannerHash      string                     `json:"effective_banner_hash"`
	EffectiveBannerURL       string                     `json:"effective_banner_url"`
	EffectiveBannerWidth     int                        `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int                        `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string                     `json:"effective_banner_thumbhash,omitempty"`
	Covers                   []NextMoeGalgameCover      `json:"covers"`
	Screenshots              []NextMoeGalgameScreenshot `json:"screenshots"`
	Alias                    []NextMoeAlias             `json:"alias"`
	Official                 []NextMoeOfficialRel       `json:"official"`
	Engine                   []NextMoeEngineWithAlias   `json:"engine"`
	Series                   []NextMoeSeriesRef         `json:"series"`
	Tag                      []NextMoeTagWithSpoiler    `json:"tag"`
	Contributor              []NextMoeContributor       `json:"contributor"`
	Created                  string                     `json:"created"`
	Updated                  string                     `json:"updated"`
	// Refs are the work's external identities keyed by catalog source
	// ("dlsite" → the DLsite workno). Carried through so the service can build the
	// 正版购买 link for the galgame header.
	Refs map[string]string `json:"refs,omitempty"`
}

// NextMoeSeriesRef is one series a work belongs to: identity only, because the
// series page renders its own forum-local count and a second number attached to
// the link would disagree with it.
type NextMoeSeriesRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// NextMoeEngineWithAlias matches the engine-embedded-in-galgame shape (alias is []string).
//
// `galgame_count` source matches NextMoeOfficial — galgame injects the count
// subquery into the detail-time Preload so the "+N" chip can be filled
// without a follow-up request.
type NextMoeEngineWithAlias struct {
	Engine struct {
		ID           int      `json:"id"`
		Name         string   `json:"name"`
		Alias        []string `json:"alias"`
		GalgameCount int      `json:"galgame_count"`
	} `json:"engine"`
}

// NextMoeGalgameDetailFullResp is the envelope with galgame + users map.
type NextMoeGalgameDetailFullResp struct {
	Galgame NextMoeGalgameDetailFull `json:"galgame"`
	Users   map[string]NextMoeUser   `json:"users"`
}

// NextMoeSeriesSample is a sample galgame inside a series detail response.
//
// EffectiveBannerHash/URL must be declared here even though galgame's series
// payload nests these objects — rewriteBanners walks the JSON and injects
// `effective_banner_url`, but Go's json.Unmarshal silently drops any field
// not declared on the target struct, leaving the FE carousel without the
// U2 hash/URL pair for newly-uploaded (covers-only) galgames.
type NextMoeSeriesSample struct {
	NameEnUs                 string `json:"name_en_us"`
	NameJaJp                 string `json:"name_ja_jp"`
	NameZhCn                 string `json:"name_zh_cn"`
	NameZhTw                 string `json:"name_zh_tw"`
	Banner                   string `json:"banner"`
	ContentLimit             string `json:"content_limit"`
	EffectiveBannerHash      string `json:"effective_banner_hash"`
	EffectiveBannerURL       string `json:"effective_banner_url"`
	EffectiveBannerWidth     int    `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int    `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string `json:"effective_banner_thumbhash,omitempty"`
}

// NextMoeSeriesBrief is the shape of /series/:id used inside GalgameDetail.
type NextMoeSeriesBrief struct {
	ID          int                   `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Galgame     []NextMoeSeriesSample `json:"galgame"`
	Created     string                `json:"created"`
	Updated     string                `json:"updated"`
}

// NextMoeCreatedResp is the shape returned by POST /galgame (just the ID).
type NextMoeCreatedResp struct {
	ID int `json:"id"`
}
