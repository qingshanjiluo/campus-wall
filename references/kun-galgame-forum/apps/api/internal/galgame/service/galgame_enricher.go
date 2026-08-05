package service

import (
	"context"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/pkg/userclient"
)

// GalgameEnricher turns galgame galgame items into the enriched GalgameCard
// shape the frontend consumes, applying NSFW filtering and fusing in local
// interaction counts + user info.
//
// This is the single source of truth for "galgame galgame + local enrichment"
// across series / official / engine / tag detail endpoints.
type GalgameEnricher struct {
	galgameRepo *repository.GalgameRepository
	metaRepo    *repository.GalgameResourceMetaRepository
	userClient  *userclient.Client
}

func NewGalgameEnricher(
	galgameRepo *repository.GalgameRepository,
	metaRepo *repository.GalgameResourceMetaRepository,
	userClient *userclient.Client,
) *GalgameEnricher {
	return &GalgameEnricher{galgameRepo: galgameRepo, metaRepo: metaRepo, userClient: userClient}
}

// Samples returns up to `n` minimal samples (name + banner).
func (e *GalgameEnricher) Samples(items []dto.NextMoeGalgameItem, n int) []dto.GalgameSample {
	if n > len(items) {
		n = len(items)
	}
	out := make([]dto.GalgameSample, 0, n)
	for i := 0; i < n; i++ {
		g := items[i]
		out = append(out, dto.GalgameSample{
			Name: dto.KunLanguage{
				EnUs: g.NameEnUs, JaJp: g.NameJaJp,
				ZhCn: g.NameZhCn, ZhTw: g.NameZhTw,
			},
			Banner:                   g.Banner,
			EffectiveBannerHash:      g.EffectiveBannerHash,
			EffectiveBannerURL:       g.EffectiveBannerURL,
			EffectiveBannerWidth:     g.EffectiveBannerWidth,
			EffectiveBannerHeight:    g.EffectiveBannerHeight,
			EffectiveBannerThumbhash: g.EffectiveBannerThumbhash,
		})
	}
	return out
}

// ToCards converts galgame galgame items into enriched GalgameCard DTOs, batch-
// loading users (from OAuth) and local stats once.
func (e *GalgameEnricher) ToCards(ctx context.Context, items []dto.NextMoeGalgameItem) []dto.GalgameCard {
	if len(items) == 0 {
		return []dto.GalgameCard{}
	}

	galgameIDs := make([]int, len(items))
	for i, g := range items {
		galgameIDs[i] = g.ID
	}

	// Local stats batch — also the source of the author chip: the wiki-era
	// creator is frozen on the local row (migration 066) because the catalog
	// face carries no submitter (doc 106 R2 ②), which is why the users are
	// hydrated from the local rows and NOT from the item's own (always-zero)
	// UserID. Must therefore run BEFORE the Hydrate call below.
	localMap := e.galgameRepo.FindLocalBatch(galgameIDs)
	userMap := e.userClient.Hydrate(ctx, frozenCreatorIDs(galgameIDs, localMap))
	// Platform/language badges come from the LOCAL galgame_resource rows — the
	// galgame item (incl. Meilisearch search hits) carries neither — so the search /
	// series / official / engine / tag cards match the /galgame list card's
	// top-left platform badges. Cheap now that galgame_resource(galgame_id) is indexed.
	platformMap, languageMap := groupResourceMeta(e.metaRepo.FindResourceMetaBatch(galgameIDs))

	cards := make([]dto.GalgameCard, len(items))
	for i, g := range items {
		// Present in FindLocalBatch ⇒ the forum has a local row for this game
		// (created/claimed/approved or has activity). Galgame-only games are absent
		// ⇒ IsOnForum=false and their view/like/platform/language stay zero/empty
		// (the frontend hides those rather than showing misleading zeros).
		_, onForum := localMap[g.ID]
		cards[i] = dto.GalgameCard{
			ID: g.ID,
			Name: dto.KunLanguage{
				EnUs: g.NameEnUs, JaJp: g.NameJaJp,
				ZhCn: g.NameZhCn, ZhTw: g.NameZhTw,
			},
			Banner:       g.Banner,
			User:         frozenCreatorBrief(localMap[g.ID], userMap),
			ContentLimit: g.ContentLimit,
			// View is a kungal-local stat (each site has its own audience),
			// not metadata; pull from the local stats row instead of galgame.
			View:               localMap[g.ID].View,
			LikeCount:          localMap[g.ID].LikeCount,
			ResourceUpdateTime: g.ResourceUpdateTime,
			ReleaseDate:        g.ReleaseDate,
			ReleaseDateTBA:     g.ReleaseDateTBA,
			// "" for non-calendar sources (galgame only emits it on calendar
			// endpoints) — the calendar FE reads it; other pages ignore it.
			ReleasePrecision: g.ReleasePrecision,
			// 2 = unclaimed VNDB draft (calendar only) → FE renders a 未发布
			// claim card. 0 (published) everywhere else.
			Status: g.Status,
			// U2: card carries only the derived banner; cdn_url/
			// effective_banner_url is injected by client.rewriteBanners
			// walker. banner_image_hash retired in galgame PR5 (K-PR6).
			EffectiveBannerHash:      g.EffectiveBannerHash,
			EffectiveBannerURL:       g.EffectiveBannerURL,
			EffectiveBannerWidth:     g.EffectiveBannerWidth,
			EffectiveBannerHeight:    g.EffectiveBannerHeight,
			EffectiveBannerThumbhash: g.EffectiveBannerThumbhash,
			Platform:                 emptyStrSliceIfNil(platformMap[g.ID]),
			Language:                 emptyStrSliceIfNil(languageMap[g.ID]),
			IsOnForum:                onForum,
		}
	}
	return cards
}
