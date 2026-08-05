package client

// Catalog public-face wire DTOs + projections (A2-3 re-anchoring).
//
// The kungal galgame READ set moves off the deprecated `/v1/galgame` product
// face onto the canonical `/v1/catalog` registry face. This file holds the
// catalog wire structs the client parses and the projections back onto the
// kungal-internal shapes (GalgameBrief / NextMoeGalgameItem / …) the services
// already consume — so kungal's OWN API output stays shape-stable and the blast
// radius stops at this package.
//
// Three conversions carry real semantics and are therefore named functions with
// their reasoning attached rather than inline expressions:
//
//   - claimed_by.content_limit → the kungal sfw/nsfw DISPLAY binary, with the
//     age rating as the fallback only (contentLimitOf)
//   - olang (BCP-47) → the kungal product locale keys (productLocale)
//   - claimed_by.state → the card's lifecycle badge (statusFromClaimState)
//
// What the catalog face deliberately does NOT carry is the wiki's own state
// machine (submitter, draft status). Ownership now comes from the surviving
// /internal meta op (permission / notification lanes) or from the frozen local
// creator snapshot (display lanes) — never from here. See doc 106 §26 R2.

import (
	"strings"

	"kun-galgame-api/internal/galgame/dto"
)

// ─── catalog wire structs (only the fields this client reads) ─────────────

// catClaimedBy is the cross-face content pointer. WorkID is the kungal gid
// whenever Site == ClaimSiteKungal — the ONLY bridge between the catalog id
// space and kungal's own primary key (doc 106 R3: no dual-id model).
//
// State is the catalog-owned claim visibility (live | draft | hidden), NOT the
// wiki status machine. A hidden row means the product withdrew the entry; it
// must never render (that is the finding R7 exists for).
//
// ContentLimit is the CLAIMING product's own editorial verdict on the entry
// (sfw | nsfw) — the wiki-body axis kungal's noindex / blur / 确认显示 gate has
// always read. Every claim carries it; an unclaimed work has no claimed_by at
// all and therefore no verdict (see contentLimitOf).
type catClaimedBy struct {
	Site         string `json:"site"`
	WorkID       int    `json:"work_id"`
	State        string `json:"state"`
	ContentLimit string `json:"content_limit"`
}

// catRef is one exact cross-source identity anchor.
type catRef struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
}

// catCoverSlot is one filled cover slot of include=covers — a complete CDN URL
// plus the intrinsic display metadata (absent when image_service has no entry).
type catCoverSlot struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Thumbhash string `json:"thumbhash"`
	Sexual    int    `json:"sexual"`
	Violence  int    `json:"violence"`
	Source    string `json:"source"`
}

// catCoverSlots is the two-slot cover projection. Either may be null.
type catCoverSlots struct {
	Portrait *catCoverSlot `json:"portrait"`
	Banner   *catCoverSlot `json:"banner"`
}

// catNames is the D7 four-key title pivot (include=names). Keys are omitted
// (not nulled) when the work has no title for them.
type catNames struct {
	JaJP string `json:"ja-jp"`
	ZhCN string `json:"zh-cn"`
	ZhTW string `json:"zh-tw"`
	EnUS string `json:"en-us"`
}

// catIntroSlot is one product key's intro with its provenance.
type catIntroSlot struct {
	Intro   string `json:"intro"`
	Source  string `json:"source"`
	Machine bool   `json:"machine"`
}

// catIntros is the D7 four-key intro pivot (include=intros).
type catIntros struct {
	JaJP *catIntroSlot `json:"ja-jp"`
	ZhCN *catIntroSlot `json:"zh-cn"`
	ZhTW *catIntroSlot `json:"zh-tw"`
	EnUS *catIntroSlot `json:"en-us"`
}

// catWorkLabel is one label attribution edge (include=labels / detail labels[]).
//
// LabelKind is the label's OWN nature (game_brand / doujin_circle / …) and Kind
// is its role on THIS work (developer / publisher / …) — one label may attach
// through several roles, so the same id can appear more than once here.
//
// Lang is the display name's own language tag; WorkCount is the nsfw-aware
// number of works the same caller would page through via works?label_id= (both
// A2-R2 supply). WorkCount has no omitempty on the wire and is absent only
// while the supplying wave is undeployed — which lands on 0, the value the
// consumer treats as "no badge".
type catWorkLabel struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	LabelKind   string `json:"label_kind"`
	Kind        string `json:"kind"`
	Lang        string `json:"lang"`
	WorkCount   int    `json:"work_count"`
}

// catWorkEngine is one engine attribution (detail engines[], A2-1e).
// WorkCount is the same nsfw-aware count as works?engine_id= (A2-R2).
type catWorkEngine struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	WorkCount int    `json:"work_count"`
}

// catWorkLink is one non-identity external web link (detail links[], A2-1e).
// There is no title key by construction — the retirement wave absorbed the URL
// bytes without the user-typed caption, so consumers render the source key.
type catWorkLink struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

// catRating is one source-native rating (scale is source-native).
type catRating struct {
	Source    string  `json:"source"`
	Score     float64 `json:"score"`
	VoteCount int     `json:"vote_count"`
}

// CatalogWorkListItem is one row of the works browse / search / calendar lanes
// (they are the same row by construction — doc 106 A2-1c). The include= blocks
// are nil unless their token was requested.
type CatalogWorkListItem struct {
	ID            int64         `json:"id"`
	Medium        string        `json:"medium"`
	DisplayName   string        `json:"display_name"`
	ContentRating string        `json:"content_rating"`
	OLang         string        `json:"olang"`
	ReleaseDate   *string       `json:"release_date"`
	ClaimedBy     *catClaimedBy `json:"claimed_by"`
	Cover         string        `json:"cover"`
	Updated       string        `json:"updated"`

	Names   *catNames      `json:"names"`
	Intros  *catIntros     `json:"intros"`
	Labels  []catWorkLabel `json:"labels"`
	Ratings []catRating    `json:"ratings"`
	Covers  *catCoverSlots `json:"covers"`
	Refs    []catRef       `json:"refs"`
}

// catWorksListData is the keyset works-list / calendar envelope.
type catWorksListData struct {
	Items      []CatalogWorkListItem `json:"items"`
	NextCursor *string               `json:"next_cursor"`
	Count      int64                 `json:"count"`
	Month      string                `json:"month"`
	Year       string                `json:"year"`
	Meta       catCalendarMeta       `json:"meta"`
}

// catCalendarMeta is the calendar navigation frame (A2-1e / R10). HasPrev /
// HasNext are pointers because they are month-bucket only.
type catCalendarMeta struct {
	Today    string `json:"today"`
	MinMonth string `json:"min_month"`
	MaxMonth string `json:"max_month"`
	HasPrev  *bool  `json:"has_prev"`
	HasNext  *bool  `json:"has_next"`
}

// catWorksSearchData is the product-search envelope (page-paginated, unlike the
// keyset browse lane).
type catWorksSearchData struct {
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Items []CatalogWorkListItem `json:"items"`
}

// ─── shared helpers ──────────────────────────────────────────────────────

// hashFromURL extracts the content-addressed image hash from a sharded CDN URL
// ({base}/aa/bb/<hash>.webp): the basename minus the extension. Several kungal
// DTOs still publish the bare hash alongside the URL. Returns "" for an empty
// URL (no image).
func hashFromURL(u string) string {
	if u == "" {
		return ""
	}
	base := u
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".webp")
}

// zeroTS is the JSON rendering of a Go zero time.Time — the value kungal's
// wire has always carried for an unset resource_update_time. The catalog has no
// counterpart for that wiki field (it never described kungal activity anyway;
// the local row owns it), so the projections keep emitting this literal rather
// than changing the key's shape.
const zeroTS = "0001-01-01T00:00:00Z"

// ─── semantic conversions ────────────────────────────────────────────────

// contentLimitOf resolves the kungal DISPLAY binary for one row: the claiming
// product's own editorial verdict wins, the age-axis mapping is the fallback.
//
// The two axes answer different questions and conflating them was doc 106 §38:
//
//	content_rating (catalog) — how old you must be to play the GAME. 94.5% of
//	                           the registry is r18: that is the medium, not a
//	                           statement about kungal's page for it.
//	content_limit  (wiki)    — whether the ENTRY's own display material is
//	                           sanitized. This is what kungal's noindex / blur /
//	                           确认显示 gate has always been keyed on; 4,812 of
//	                           the 10,929 published entries carry the nsfw
//	                           verdict — a set the age axis misstates in BOTH
//	                           directions (5,568 sfw entries are rated r18, 50
//	                           nsfw entries are rated all_ages).
//
// Driving content_limit off content_rating therefore marked 94.5% of the
// catalogue nsfw — which noindexed the entire galgame surface (6,117 → 599
// indexed pages) while simultaneously un-marking the entries an editor HAD
// flagged, because their age rating happened to be all_ages.
//
// The fallback stays conservative rather than optimistic: an unclaimed work has
// no editorial verdict at all (no claimed_by), and neither does a claim written
// by a catalog that has not shipped the key yet — for both the age axis is the
// only signal available, which is the pre-wave behaviour.
func contentLimitOf(claimed *catClaimedBy, rating string) string {
	if claimed != nil {
		// A closed word list, not "non-empty wins": an unrecognised value would
		// otherwise flow onto kungal's wire, where every consumer compares it
		// against the literal "nsfw" and would read the garbage as sfw.
		switch claimed.ContentLimit {
		case "sfw", "nsfw":
			return claimed.ContentLimit
		}
	}
	return contentLimitFromRating(rating)
}

// contentLimitFromRating is contentLimitOf's FALLBACK — the age-axis reading,
// used only where no editorial verdict exists. `sensitive` folds to sfw
// deliberately: it is the rating for suggestive-but-not-adult works, which the
// wiki also carried as content_limit=sfw. Folding it to nsfw instead would hide
// works that are visible today.
func contentLimitFromRating(rating string) string {
	if rating == "r18" {
		return "nsfw"
	}
	return "sfw"
}

// ageLimitFromRating maps the content rating onto the wiki's age_limit
// vocabulary ("all" / "r18"), which several kungal DTOs still publish.
func ageLimitFromRating(rating string) string {
	if rating == "r18" {
		return "r18"
	}
	return "all"
}

// productLocale projects a catalog BCP-47 original-language tag onto the kungal
// product locale keys, the D7 table ① mapping (docs/developer-platform
// 02-public-api.md §3.2.1). A tag outside the four product locales passes
// through VERBATIM rather than being blanked: `original_language` is a display
// label here, and showing "ko" is honest where showing "" is a silent loss.
func productLocale(olang string) string {
	tag := strings.ToLower(strings.TrimSpace(olang))
	switch {
	case tag == "":
		return ""
	case tag == "ja" || strings.HasPrefix(tag, "ja-"):
		return "ja-jp"
	case tag == "zh-hant" || strings.HasPrefix(tag, "zh-hant-") ||
		tag == "zh-tw" || tag == "zh-hk":
		return "zh-tw"
	case tag == "zh" || strings.HasPrefix(tag, "zh"):
		return "zh-cn"
	case tag == "en" || strings.HasPrefix(tag, "en-"):
		return "en-us"
	default:
		return olang
	}
}

// Claim-state projections. The kungal card renders three lifecycle cases and
// they now come from claimed_by, not from the retired wiki status machine:
//
//	live          → a published entry            → status 0, links to /galgame/{gid}
//	draft         → an entry the wiki has not published → status 2, "未在论坛发布" claim card
//	hidden        → the wiki WITHDREW it         → dropped entirely (never rendered)
//	claimed_by nil → not on the wiki at all      → status 2 as well: from kungal's
//	                 side "there is no forum entry for this work" is the same
//	                 statement, and the claim card is exactly the right CTA.
const (
	claimStateLive    = "live"
	claimStateDraft   = "draft"
	claimStatePending = "pending"
	claimStateHidden  = "hidden"
)

// ClaimStateWizard is the claim_state filter for lanes that want every entry
// the registry HAS, published or not — i.e. the publish wizard's dedup supply.
// Public browse lanes want `live` alone (doc 106 §37); this one is the
// deliberate exception.
//
// `pending` is listed even though the projector does not produce it yet: the
// wiki-era projection folded "unclaimed draft" and "awaiting review" onto
// `draft`, and the fix that separates them lands separately. Asking for the
// word first is what makes the two independent — today it matches nothing, and
// the day it starts matching, the wizard already knows how to label it.
const ClaimStateWizard = claimStateLive + "," + claimStateDraft + "," + claimStatePending

// statusFromClaimState maps a claim state onto the kungal card status code.
func statusFromClaimState(state string) int {
	if state == claimStateLive {
		return galgameStatusPublished
	}
	return galgameStatusVndbDraft
}

// kungal card status codes (the FE GalgameStatus enum).
const (
	galgameStatusPublished = 0
	galgameStatusVndbDraft = 2
)

// isRenderable reports whether a catalog row may be shown at all. A hidden
// claim means the owning product took the entry down — re-anchoring on
// claimed_by without this check silently republishes banned entries, which is
// the single sharpest finding of the A2-2 report (doc 106 §27 S1 / R7).
func (it *CatalogWorkListItem) isRenderable() bool {
	return it.ClaimedBy == nil || it.ClaimedBy.State != claimStateHidden
}

// CatalogItemRenderable is isRenderable for callers outside this package: the
// service layer projects raw pages (search / calendar) and must apply the same
// withdrawal gate the batch bridge applies internally.
func CatalogItemRenderable(it *CatalogWorkListItem) bool { return it.isRenderable() }

// CatalogItemGID is gid() for callers outside this package: 0 when the work has
// no kungal entry. Never substitute the catalog id — the two key spaces overlap
// (doc 106 R3).
func CatalogItemGID(it *CatalogWorkListItem) int { return it.gid() }

// gid returns the kungal galgame id this catalog row is claimed by, or 0 when
// the work has no kungal entry. NEVER fall back to the catalog id here: the two
// key spaces overlap, and a catalog id used as a gid would attach another
// game's local stats (doc 106 R3).
func (it *CatalogWorkListItem) gid() int {
	if it.ClaimedBy == nil || !isKungalClaim(it.ClaimedBy.Site) {
		return 0
	}
	return it.ClaimedBy.WorkID
}

// ClaimSiteKungal is the catalog CLAIM SITE key for kungal's galgame entries —
// the value `claimed_by.site` carries and the tenant lifecycle actions are
// filed under. This is the value kungal WRITES.
//
// It is not the external-ref source key (client.anchorSourceKeys), though the
// two spelled `galgame_wiki` alike for as long as the wiki was both the
// claiming product and the provenance of the bridged rows. They stop agreeing
// in the W1 window, and in different steps.
const ClaimSiteKungal = "kungal"

// claimSiteLegacy is what `claimed_by.site` reads until the re-site step
// rewrites it. Recognising both is what keeps the reader correct on either
// side of that step, so the flip needs no coordinated deploy: a constant
// matching only the new value would return gid 0 for every row until the data
// caught up, and a gid of 0 does not raise anything — it just quietly removes
// every card's link and every entry's local stats.
//
// Removable once the re-site has soaked. It is only ever READ; nothing writes
// it.
const claimSiteLegacy = "galgame_wiki"

func isKungalClaim(site string) bool {
	return site == ClaimSiteKungal || site == claimSiteLegacy
}

// refsMap flattens the exact identity anchors into the open source→id map the
// kungal briefs carry (kungal reads "dlsite" for the 补票 purchase link).
// Returns nil when the block was not requested, so an absent include=refs is
// distinguishable from a work with no anchors.
func refsMap(refs []catRef) map[string]string {
	if refs == nil {
		return nil
	}
	out := make(map[string]string, len(refs))
	for _, r := range refs {
		prev, seen := out[r.Source]
		switch {
		case !seen:
			out[r.Source] = r.ExternalID
		case r.Source == sourceVNDB && !isVndbWorkID(prev) && isVndbWorkID(r.ExternalID):
			// VNDB is the one source whose anchors are not interchangeable: a
			// work commonly carries BOTH a release anchor (r69531) and the work
			// anchor (v27920), and only the latter is what kungal means by
			// "the VNDB id" — it is the one the wizard prints and the one every
			// VNDB link is built from. It wins regardless of arrival order.
			out[r.Source] = r.ExternalID
		}
		// Otherwise first anchor per source wins: a work can carry several
		// release-level anchors of one source (dlsite digital + physical) and
		// the product links to one of them.
	}
	return out
}

const sourceVNDB = "vndb"

// isVndbWorkID reports whether an external id is a vndb WORK anchor: `v`
// followed by at least one digit and nothing else. Stricter than a prefix test
// on purpose — `v`, `v12a` and `vn3` are not work ids.
func isVndbWorkID(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// coverFields projects the two-slot cover block onto the derived
// effective-banner quintet the kungal briefs / items carry.
//
// The BANNER slot wins and portrait is the fallback: kungal's card and hero
// frames are landscape, so a portrait pin is letterboxed or cropped into them.
// (Ruled by the user after the re-anchoring shipped portrait-first — the
// preference is for the wide banner art wherever a work has one.) A
// portrait-only work still renders through the fallback rather than showing a
// blank card, and `cover` — the list lane's single representative URL — remains
// the last resort for lanes that did not ask for include=covers.
func coverFields(covers *catCoverSlots, fallbackURL string) (hash, url string, w, h int, thumb string) {
	slot := (*catCoverSlot)(nil)
	if covers != nil {
		if covers.Banner != nil {
			slot = covers.Banner
		} else if covers.Portrait != nil {
			slot = covers.Portrait
		}
	}
	if slot == nil {
		return hashFromURL(fallbackURL), fallbackURL, 0, 0, ""
	}
	return hashFromURL(slot.URL), slot.URL, slot.Width, slot.Height, slot.Thumbhash
}

// ─── projections onto the kungal-internal shapes ─────────────────────────

// CatalogItemToBrief projects a catalog list row onto the GalgameBrief the
// enrichers consume. The brief is keyed by the kungal gid, never by the catalog
// id (R3). UserID is left zero here — display ownership comes from the frozen
// local creator snapshot, which the service layer fuses in; permission and
// notification lanes read the /internal meta op instead.
func CatalogItemToBrief(it *CatalogWorkListItem) GalgameBrief {
	b := GalgameBrief{
		ID:               it.gid(),
		NameEnUs:         it.name("en-us"),
		NameJaJp:         it.name("ja-jp"),
		NameZhCn:         it.name("zh-cn"),
		NameZhTw:         it.name("zh-tw"),
		AgeLimit:         ageLimitFromRating(it.ContentRating),
		ContentLimit:     contentLimitOf(it.ClaimedBy, it.ContentRating),
		OriginalLanguage: productLocale(it.OLang),
		ReleaseDate:      it.ReleaseDate,
		// The wiki's own resource_update_time has no catalog counterpart and
		// never described kungal activity anyway; the local row owns it (the
		// list already sorted and labelled by the LOCAL value). Kept at the
		// bridge-parity zero literal so the JSON key shape is unchanged.
		ResourceUpdateTime: zeroTS,
		Refs:               refsMap(it.Refs),
	}
	if it.ClaimedBy != nil {
		b.Status = statusFromClaimState(it.ClaimedBy.State)
	} else {
		b.Status = galgameStatusVndbDraft
	}
	b.VndbID = b.Refs["vndb"]
	b.EffectiveBannerHash, b.EffectiveBannerURL,
		b.EffectiveBannerWidth, b.EffectiveBannerHeight,
		b.EffectiveBannerThumbhash = coverFields(it.Covers, it.Cover)
	return b
}

// name reads one product-locale title off the include=names block, falling back
// to the registry display_name when the block was not requested. The catalog
// omits an empty key, so a missing locale is "" — the kungal convention.
func (it *CatalogWorkListItem) name(key string) string {
	if it.Names == nil {
		// Without include=names the only title available is the registry's
		// display_name; put it in the ja-jp slot, which is the kungal fallback
		// chain's Japanese rung, rather than leaving every name blank.
		if key == "ja-jp" {
			return it.DisplayName
		}
		return ""
	}
	switch key {
	case "ja-jp":
		return it.Names.JaJP
	case "zh-cn":
		return it.Names.ZhCN
	case "zh-tw":
		return it.Names.ZhTW
	case "en-us":
		return it.Names.EnUS
	}
	return ""
}

// intro reads one product-locale intro off the include=intros block.
func (it *CatalogWorkListItem) intro(key string) string {
	if it.Intros == nil {
		return ""
	}
	slot := (*catIntroSlot)(nil)
	switch key {
	case "ja-jp":
		slot = it.Intros.JaJP
	case "zh-cn":
		slot = it.Intros.ZhCN
	case "zh-tw":
		slot = it.Intros.ZhTW
	case "en-us":
		slot = it.Intros.EnUS
	}
	if slot == nil {
		return ""
	}
	return slot.Intro
}

// CatalogItemToDetailBrief is CatalogItemToBrief plus the intro + maker names
// the richer feed card renders (include=intros,labels).
func CatalogItemToDetailBrief(it *CatalogWorkListItem) GalgameDetailBrief {
	b := GalgameDetailBrief{GalgameBrief: CatalogItemToBrief(it)}
	b.IntroEnUS = it.intro("en-us")
	b.IntroJaJP = it.intro("ja-jp")
	b.IntroZhCN = it.intro("zh-cn")
	b.IntroZhTW = it.intro("zh-tw")
	for _, l := range it.Labels {
		b.Officials = append(b.Officials, l.DisplayName)
	}
	return b
}

// CatalogItemToNextMoeItem projects a catalog list row onto the
// dto.NextMoeGalgameItem the enricher / calendar / search adapters consume.
//
// ID is the kungal gid when the work is claimed and 0 otherwise — the enricher
// treats 0 as "no local row", which is exactly right for a catalog work kungal
// has never ingested (it renders the 未在论坛发布 claim card, whose link is
// name-based, so it needs no id).
func CatalogItemToNextMoeItem(it *CatalogWorkListItem) dto.NextMoeGalgameItem {
	m := dto.NextMoeGalgameItem{
		ID:           it.gid(),
		NameEnUs:     it.name("en-us"),
		NameJaJp:     it.name("ja-jp"),
		NameZhCn:     it.name("zh-cn"),
		NameZhTw:     it.name("zh-tw"),
		ReleaseDate:  it.ReleaseDate,
		ContentLimit: contentLimitOf(it.ClaimedBy, it.ContentRating),
		// release_precision is self-describing on the catalog wire: the partial
		// ISO date's LENGTH is the precision (4 = year, 7 = month, 10 = day),
		// per D7 table ②. Projecting it back to the old keyword keeps the
		// calendar FE unchanged.
		ReleasePrecision: releasePrecisionOf(it.ReleaseDate),
	}
	if it.ClaimedBy != nil {
		m.Status = statusFromClaimState(it.ClaimedBy.State)
	} else {
		m.Status = galgameStatusVndbDraft
	}
	m.EffectiveBannerHash, m.EffectiveBannerURL,
		m.EffectiveBannerWidth, m.EffectiveBannerHeight,
		m.EffectiveBannerThumbhash = coverFields(it.Covers, it.Cover)
	return m
}

// releasePrecisionOf recovers the old face's precision keyword from a partial
// ISO date. nil (no dated release) is "tba" — the deprecated face used that
// keyword for a work with an announced-but-undated release, which is exactly
// the population of the catalog's tba bucket.
func releasePrecisionOf(date *string) string {
	if date == nil {
		return "tba"
	}
	switch len(*date) {
	case 4:
		return "year"
	case 7:
		return "month"
	case 10:
		return "day"
	}
	return ""
}
