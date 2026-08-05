package client

// Projection of the catalog aggregate work record onto the kungal detail DTO.
//
// Kept apart from catalog_wire.go because the detail face is a genuinely
// different shape: the list lane pivots titles/intros to the four product keys
// server-side, while the detail lane ships the COMPLETE language set and the
// pivot happens here.
//
// Three blocks changed owner in this wave and the reasoning is worth keeping
// next to the code:
//
//   - engines[] is now a first-class detail block (A2-1e). Since A2-R2 every
//     attribution row — labels, tags and engines alike — carries the same
//     nsfw-aware work_count the INDEX pages count with, so the "+N" chip is
//     filled straight off the aggregate. The key is tolerated as MISSING: an
//     unmapped source tag never has one, and no row has one until the supplying
//     wave is deployed. Both decode to 0, which the FE renders as no badge at
//     all rather than as "+ 0".
//   - links[] carries {source, url} and no title: the retirement wave absorbed
//     the wiki's user-typed captions away, so the FE renders the source key.
//     The per-row user_id that drove the banned-author filter is gone with them
//     by ruling (doc 126 D6) — these are platform-curated rows now, with no
//     author to ban.
//   - contributors are not projected at all: wiki-era contribution froze with
//     the wiki (D6), and the catalog has no forward contribution feed yet. The
//     service renders the author alone.

import (
	"context"
	"strings"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// CatalogDetailToFull projects the catalog aggregate onto the shape
// galgameDetailFromNextMoe consumes. gid is passed in rather than read off
// claimed_by: the caller looked the work up BY gid, so it is authoritative,
// and it keeps the projection total even for a work whose claim row is being
// rewritten concurrently.
func CatalogDetailToFull(d *catWorkDetail, gid int) dto.NextMoeGalgameDetailFull {
	f := dto.NextMoeGalgameDetailFull{
		ID:               gid,
		ContentLimit:     contentLimitOf(d.ClaimedBy, d.ContentRating),
		AgeLimit:         ageLimitFromRating(d.ContentRating),
		OriginalLanguage: productLocale(d.OLang),
		ReleaseDate:      d.ReleaseDate,
		// The catalog models "announced but undated" as membership of the TBA
		// calendar bucket, not as a per-work flag; a work with no dated release
		// simply has release_date null. Nothing on the detail page reads the
		// flag independently of the date, so it stays false.
		ReleaseDateTBA:     false,
		Updated:            d.Updated,
		Created:            d.Created,
		ResourceUpdateTime: zeroTS,
		Refs:               refsMap(d.Refs),
		Covers:             catalogCoversToNextMoe(d),
		Screenshots:        catalogScreenshotsToNextMoe(d),
		// Wiki-era contribution froze with the wiki (D6). The author still
		// renders; the contributor strip does not.
		Contributor: []dto.NextMoeContributor{},
	}
	f.VndbID = f.Refs["vndb"]

	names, aliases := catalogTitles(d)
	f.NameJaJp, f.NameZhCn, f.NameZhTw, f.NameEnUs = names[0], names[1], names[2], names[3]
	if f.NameJaJp == "" && f.NameZhCn == "" && f.NameZhTw == "" && f.NameEnUs == "" {
		f.NameJaJp = d.DisplayName
	}
	f.Alias = aliases

	f.IntroJaJp, f.IntroZhCn, f.IntroZhTw, f.IntroEnUs = catalogIntros(d)

	if d.ClaimedBy != nil {
		f.Status = statusFromClaimState(d.ClaimedBy.State)
	}

	// Effective banner: the wide banner art wins, the portrait pin is the
	// fallback — the same preference coverFields encodes for the list lane, so a
	// game's card and its hero resolve to the same image.
	if c := detailBannerCover(f.Covers); c != nil {
		f.EffectiveBannerHash = c.ImageHash
		f.EffectiveBannerURL = c.CDNURL
		f.EffectiveBannerWidth = c.Width
		f.EffectiveBannerHeight = c.Height
		f.EffectiveBannerThumbhash = c.Thumbhash
	}

	// labels[] is one row per attribution EDGE, so a brand that both developed
	// and published a work arrives two or three times. Fold to one row per label
	// id and COLLECT the roles onto it, rather than dropping every edge after
	// the first: "开发商 · 发行商" is the fact a reader wants, and it is the whole
	// reason the registry models the edge kind separately from the label's own
	// kind. Rendering the name three times was the thing to avoid; losing two
	// thirds of the attribution was too high a price for it.
	//
	// The face orders edges by kind, so a given label's role list is in the same
	// order on every request.
	labelAt := make(map[int64]int, len(d.Labels))
	for _, l := range d.Labels {
		if i, seen := labelAt[l.ID]; seen {
			f.Official[i].Official.Roles = appendUniqueStr(f.Official[i].Official.Roles, l.Kind)
			continue
		}
		labelAt[l.ID] = len(f.Official)
		f.Official = append(f.Official, dto.NextMoeOfficialRel{Official: dto.NextMoeOfficial{
			ID: int(l.ID), Name: l.DisplayName,
			// Category is the label's OWN kind (game_brand / doujin_circle / …),
			// the vocabulary the 会社 index renders. Roles is what it DID on this
			// work. Two different questions, so two fields — collapsing them is
			// what made the role unrenderable in the first place.
			Category:     l.LabelKind,
			Roles:        appendUniqueStr(nil, l.Kind),
			Lang:         l.Lang,
			Alias:        []dto.NextMoeAlias{},
			GalgameCount: l.WorkCount,
		}})
	}
	for _, sr := range d.Series {
		f.Series = append(f.Series, dto.NextMoeSeriesRef{ID: int(sr.ID), Name: sr.Name})
	}
	for _, e := range d.Engines {
		var ew dto.NextMoeEngineWithAlias
		ew.Engine.ID = int(e.ID)
		ew.Engine.Name = e.Name
		ew.Engine.Alias = []string{}
		ew.Engine.GalgameCount = e.WorkCount
		f.Engine = append(f.Engine, ew)
	}
	for _, t := range d.Tags {
		// Only canonical-mapped tags become chips: an unmapped source tag has
		// no id to link to, and a chip that navigates nowhere is worse than an
		// absent one. The per-edge spoiler level rides along so the FE keeps
		// blurring spoiler tags, and the tag-level sexual flag lets the SFW
		// view drop adult vocabulary (A2-1e R8 — without it the SFW gate would
		// silently coarsen to the work-level rating).
		if t.CanonicalID == 0 {
			continue
		}
		// Hidden-tier vocabulary (platform words, site-wide truisms — junk) is
		// excluded from browse/search/picker; the detail chip strip follows the
		// same rule so a chip never links to a page every list refuses to show.
		if t.Tier == TagTierHidden {
			continue
		}
		f.Tag = append(f.Tag, dto.NextMoeTagWithSpoiler{
			SpoilerLevel: t.Spoiler,
			Tag: dto.NextMoeTag{
				ID: int(t.CanonicalID), Name: t.Name,
				Category:     catalogTagCategory(t.Kind, t.Sexual),
				GalgameCount: t.WorkCount,
			},
		})
	}
	return f
}

// detailBannerCover picks the detail hero out of the projected covers: the
// first LANDSCAPE row wins, and covers[0] — server-ordered, so the portrait pin
// — is the fallback for a work that has no wide art at all.
//
// The detail face ships a flat covers[] rather than the list lane's two slots,
// so orientation has to be derived here. It cannot come from `kind`: that is
// the VNDB cover TYPE vocabulary ("main" / "pkgfront" / "dig" / …), which says
// nothing about shape. The cutoff is the catalog's own portrait rule —
// height > width × 1.05, written as the exact rational so a near-square cover
// can never land in one slot on the card and the other on the hero.
func detailBannerCover(covers []dto.NextMoeGalgameCover) *dto.NextMoeGalgameCover {
	if len(covers) == 0 {
		return nil
	}
	for i := range covers {
		c := &covers[i]
		if c.Width > 0 && c.Height > 0 && c.Height*20 <= c.Width*21 {
			return c
		}
	}
	return &covers[0]
}

// catalogTagCategory renders the tag's category chip. The wiki's three-value
// axis (content / sexual / technical) did not migrate — the catalog models a
// content|meta `kind` plus an explicit sexual flag — so this reconstructs the
// only distinction the SFW view actually acts on and leaves the rest as the
// catalog's own vocabulary.
func catalogTagCategory(kind string, sexual bool) string {
	if sexual {
		return "sexual"
	}
	return kind
}

// catalogTitles pivots the complete title set onto the four product keys and
// collects everything else as aliases.
//
// Selection per key mirrors the catalog's own D7 rule: the lowest `kind` wins
// (official before alias before abbreviation), and titles[] arrives already
// ordered by kind, so the FIRST title seen for a key is the right one.
func catalogTitles(d *catWorkDetail) (names [4]string, aliases []dto.NextMoeAlias) {
	aliases = []dto.NextMoeAlias{}
	idx := map[string]int{"ja-jp": 0, "zh-cn": 1, "zh-tw": 2, "en-us": 3}
	for _, t := range d.Titles {
		key := productLocale(t.Lang)
		i, isProductKey := idx[key]
		if isProductKey && t.Kind == "official" && names[i] == "" {
			names[i] = t.Title
			continue
		}
		// Everything that is not the winning official title per product key is
		// an alternate spelling — which is exactly what the kungal alias strip
		// renders.
		aliases = append(aliases, dto.NextMoeAlias{Name: t.Title})
	}
	// A work with no `official` title in a product locale still deserves one:
	// promote its first alternate rather than showing a blank name.
	for _, t := range d.Titles {
		if i, ok := idx[productLocale(t.Lang)]; ok && names[i] == "" {
			names[i] = t.Title
		}
	}
	return names, aliases
}

// catalogIntros pivots the complete intro set onto the four product keys. The
// read face already merged each language to its winning source, so the first
// row per language is authoritative.
func catalogIntros(d *catWorkDetail) (jaJP, zhCN, zhTW, enUS string) {
	slots := map[string]*string{"ja-jp": &jaJP, "zh-cn": &zhCN, "zh-tw": &zhTW, "en-us": &enUS}
	for _, in := range d.Intro {
		if slot, ok := slots[productLocale(in.Lang)]; ok && *slot == "" {
			*slot = in.Intro
		}
	}
	return jaJP, zhCN, zhTW, enUS
}

// catalogCoversToNextMoe / catalogScreenshotsToNextMoe map the detail image
// blocks onto the kungal rows. sort_order is synthesized from the (server
// ordered) index, exactly as the previous projection did.
func catalogCoversToNextMoe(d *catWorkDetail) []dto.NextMoeGalgameCover {
	out := make([]dto.NextMoeGalgameCover, 0, len(d.Covers))
	for i, c := range d.Covers {
		out = append(out, dto.NextMoeGalgameCover{
			ImageHash: hashFromURL(c.URL), SortOrder: i,
			Sexual: c.Sexual, Violence: c.Violence, Kind: c.Kind,
			CDNURL: c.URL, Width: c.Width, Height: c.Height, Thumbhash: c.Thumbhash,
		})
	}
	return out
}

func catalogScreenshotsToNextMoe(d *catWorkDetail) []dto.NextMoeGalgameScreenshot {
	out := make([]dto.NextMoeGalgameScreenshot, 0, len(d.Screenshots))
	for i, s := range d.Screenshots {
		out = append(out, dto.NextMoeGalgameScreenshot{
			ImageHash: hashFromURL(s.URL), SortOrder: i, Caption: s.Caption,
			Sexual: s.Sexual, Violence: s.Violence,
			CDNURL: s.URL, Width: s.Width, Height: s.Height, Thumbhash: s.Thumbhash,
		})
	}
	return out
}

// GalgameLink is one curated external link on the detail page. It replaces the
// wiki's link row: no id and no author (the retirement wave absorbed these as
// platform-curated rows), and `name` is the source key rather than a
// user-typed caption — inventing a caption would be fabrication.
type GalgameLink struct {
	Name   string `json:"name"`
	Link   string `json:"link"`
	Source string `json:"source"`
}

// CatalogWorkLinks fetches one work's curated external links by kungal gid.
func (c *GalgameClient) CatalogWorkLinks(ctx context.Context, gid int) ([]GalgameLink, *errors.AppError) {
	d, found, appErr := c.CatalogWorkDetail(ctx, gid)
	if appErr != nil {
		return nil, appErr
	}
	out := []GalgameLink{}
	if !found {
		return out, nil
	}
	for _, l := range d.Links {
		out = append(out, GalgameLink{Name: linkDisplayName(l), Link: l.URL, Source: l.Source})
	}
	return out, nil
}

// linkDisplayName labels a link row. The source key is the honest label when it
// is a real source ("official", "twitter"); when it is empty the host is the
// next best thing a user can recognise.
func linkDisplayName(l catWorkLink) string {
	if l.Source != "" {
		return l.Source
	}
	trimmed := strings.TrimPrefix(strings.TrimPrefix(l.URL, "https://"), "http://")
	if i := strings.IndexByte(trimmed, '/'); i > 0 {
		return trimmed[:i]
	}
	return trimmed
}

// appendUniqueStr appends val unless the slice already carries it, and drops
// the empty string. A label edge with no kind contributes no role rather than a
// blank chip.
func appendUniqueStr(slice []string, val string) []string {
	if val == "" {
		return slice
	}
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
