package service

import (
	"net/url"
	"strconv"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/model"
)

// buildEntityFilter builds the local list filter for a galgame-entity detail page
// (tag / official / engine) from the request query + the entity's member ids.
// The page renders the forum-LOCAL subset of the entity's catalogue: the galgame
// supplies the member ids, and RestrictIDs scopes the SAME local filter / sort /
// paginate flow as /galgame to them, so games the forum has never ingested (no
// galgame_resource row) simply don't appear. This is why these pages can filter
// by 类型 / 语言 / 平台 / 作品类型 and sort by every field — all of which live in
// kungal-local tables the galgame knows nothing about.
//
// A non-nil (even empty) RestrictIDs means "restrict to this set", so an entity
// with no local members renders empty, never the whole catalogue (list_repo).
//
// Only the filters these pages expose are read (the advanced panel is off);
// the FE sends the same camelCase keys the browse Nav writes to the URL.
func buildEntityFilter(q url.Values, restrictIDs []int) model.GalgameListFilter {
	sortOrder := q.Get("sortOrder")
	if sortOrder == "" {
		sortOrder = "desc"
	}
	return model.GalgameListFilter{
		Type:      q.Get("type"),
		Language:  q.Get("language"),
		Platform:  q.Get("platform"),
		GameType:  q.Get("gameType"),
		SortField: q.Get("sortField"),
		SortOrder: sortOrder,
		Page:      atoiOr(q.Get("page"), 1),
		Limit:     atoiOr(q.Get("limit"), 24),
		// Honor the global "显示没有下载资源的 Galgame" setting here too (the browse
		// list already did). Off → only resourced members shown.
		ShowNoResource: q.Get("showNoResource") == "true",
		RestrictIDs:    restrictIDs,
	}
}

// listCardsToEntityCards converts the shared /galgame list cards to the entity
// detail card shape (drops the rating fields the entity card doesn't carry),
// preserving the existing entity-page response shape so the FE is unchanged.
// IsOnForum is always true here: this path lists only forum-local members (a
// game with no local row can't survive the list_repo filter), so no card is a
// galgame-only "未收录" entry.
func listCardsToEntityCards(cards []dto.GalgameListCard) []dto.GalgameCard {
	out := make([]dto.GalgameCard, len(cards))
	for i, c := range cards {
		out[i] = dto.GalgameCard{
			ID:                       c.ID,
			Name:                     c.Name,
			Banner:                   c.Banner,
			User:                     c.User,
			ContentLimit:             c.ContentLimit,
			View:                     c.View,
			LikeCount:                c.LikeCount,
			ResourceUpdateTime:       c.ResourceUpdateTime,
			Platform:                 c.Platform,
			Language:                 c.Language,
			ReleaseDate:              c.ReleaseDate,
			ReleaseDateTBA:           c.ReleaseDateTBA,
			EffectiveBannerHash:      c.EffectiveBannerHash,
			EffectiveBannerURL:       c.EffectiveBannerURL,
			EffectiveBannerWidth:     c.EffectiveBannerWidth,
			EffectiveBannerHeight:    c.EffectiveBannerHeight,
			EffectiveBannerThumbhash: c.EffectiveBannerThumbhash,
			IsOnForum:                true,
		}
	}
	return out
}

// atoiOr parses s as an int, returning fallback on any failure (empty / bad).
func atoiOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
