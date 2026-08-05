package service

import (
	"context"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/internal/moemoepoint"
	userRepo "kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/dlsite"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"
	"kun-galgame-api/pkg/utils"

	"gorm.io/gorm"
)

// GalgameService handles the "core" galgame lifecycle: create, merge PR,
// detail aggregation, list with filters, and local interaction toggles.
type GalgameService struct {
	galgameRepo      *repository.GalgameRepository
	interactionRepo  *repository.GalgameInteractionRepository
	listRepo         *repository.GalgameListRepository
	resourceMetaRepo *repository.GalgameResourceMetaRepository
	detailRatingRepo *repository.GalgameDetailRatingRepository
	stateRepo        *userRepo.StateRepository
	galgameClient    *client.GalgameClient
	userClient       *userclient.Client
	helpers          InteractionHelpers
	// DLsite affiliate wiring for the header's 正版购买 button. Empty template =
	// the button never renders.
	dlsiteLinkTemplate string
	dlsiteCouponURL    string
}

func NewGalgameService(
	galgameRepo *repository.GalgameRepository,
	interactionRepo *repository.GalgameInteractionRepository,
	listRepo *repository.GalgameListRepository,
	resourceMetaRepo *repository.GalgameResourceMetaRepository,
	detailRatingRepo *repository.GalgameDetailRatingRepository,
	stateRepo *userRepo.StateRepository,
	galgameClient *client.GalgameClient,
	userClient *userclient.Client,
	dlsiteLinkTemplate string,
	dlsiteCouponURL string,
) *GalgameService {
	return &GalgameService{
		galgameRepo:        galgameRepo,
		interactionRepo:    interactionRepo,
		listRepo:           listRepo,
		resourceMetaRepo:   resourceMetaRepo,
		dlsiteLinkTemplate: dlsiteLinkTemplate,
		dlsiteCouponURL:    dlsiteCouponURL,
		detailRatingRepo:   detailRatingRepo,
		stateRepo:          stateRepo,
		galgameClient:      galgameClient,
		userClient:         userClient,
	}
}

// ──────────────────────────────────────────
// Interactions — PUT /galgame/:gid/like|favorite
// ──────────────────────────────────────────

// ToggleLike reports an error when the user tries to self-like, otherwise
// atomically flips the like and adjusts owner moemoepoint + notification.
func (s *GalgameService) ToggleLike(
	ctx context.Context,
	userID, galgameID int,
) *errors.AppError {
	ownerID, name := s.fetchOwnerAndName(ctx, galgameID)
	if ownerID == userID {
		return errors.ErrBadRequest("您不能给自己点赞")
	}

	s.galgameRepo.DB().Transaction(func(tx *gorm.DB) error {
		liked := s.interactionRepo.ToggleLike(tx, userID, galgameID)
		if liked {
			s.helpers.AdjustMoemoepoint(tx, ownerID, 1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("galgame", galgameID))
			s.helpers.CreateGalgameMessageWithContent(tx, userID, ownerID, "liked", name, galgameID)
		} else {
			s.helpers.AdjustMoemoepoint(tx, ownerID, -1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("galgame", galgameID))
		}
		return nil
	})
	return nil
}

// NOTE: favorite is no longer a simple per-galgame toggle — it is now membership
// in one or more collections (收藏夹). The write path + owner moemoe/notification
// (first-add / last-remove semantics) live in CollectionService.SetMembership.

// GetMyInteractions returns the current user's liked + favorited galgame ids,
// for hydrating feed-card like/favorite state.
func (s *GalgameService) GetMyInteractions(userID int) dto.MyGalgameInteractions {
	liked, favorited := s.interactionRepo.UserGalgameInteractions(userID)
	return dto.MyGalgameInteractions{Liked: liked, Favorited: favorited}
}

// fetchOwnerAndName reads the galgame's owner user_id AND a display name in ONE
// request (0 / "" on any failure). The name becomes the notification content
// preview so a favorite/like notice shows WHICH galgame instead of a blank line
// — see the CreateGalgameMessageWithContent callers below.
//
// This is a PERMISSION + NOTIFICATION lane, so it reads the surviving
// /internal ownership meta op rather than the catalog: ownership is a wiki
// lifecycle fact and the catalog public face carries none by design (doc 106
// R2 ①). The op is status-blind, so the owner of an unpublished entry resolves
// too — reading it off a published-only endpoint used to return nothing and
// silently degrade to "not the owner".
//
// The fallback order zh-CN → zh-TW → ja-JP → en-US mirrors the FE's
// getPreferredLanguageText zh-cn default. en-US (usually the VNDB romaji title)
// is LAST on purpose: a JP/CN-titled game must never surface its VNDB English
// name when a Chinese or Japanese name exists.
func (s *GalgameService) fetchOwnerAndName(ctx context.Context, galgameID int) (int, string) {
	return s.ownerOf(galgameID), truncate(s.entryName(ctx, galgameID), constants.TextPreviewLength)
}

// ownerOf reads the submitter from the forum's OWN frozen snapshot
// (galgame.creator_user_id, migration 066) — the same column the author chip
// renders and the edit face's owner-review gate reads.
//
// The registry deliberately carries no submitter (doc 106 R2 ①): a registry row
// outlives any account, so owning it is not a fact about it. This used to come
// from the wiki's /internal/galgame/meta op, which retired with the wiki in
// wave-161 P5 — the column outlives the face precisely because it is the
// forum's own answer, not a borrowed one.
//
// 0 = unknown, which fails every owner check closed.
func (s *GalgameService) ownerOf(galgameID int) int {
	if s.galgameRepo == nil || galgameID <= 0 {
		return 0
	}
	row := s.galgameRepo.FindLocal(galgameID)
	if row.CreatorUserID == nil {
		return 0
	}
	return *row.CreatorUserID
}

// entryName resolves a display title for notification previews (best-effort;
// "" on any failure, which the message system renders as a blank line rather
// than a wrong name).
func (s *GalgameService) entryName(ctx context.Context, galgameID int) string {
	if s.galgameClient == nil {
		return ""
	}
	rows, appErr := s.galgameClient.CatalogRowsByGIDs(ctx, []int{galgameID}, "names", "all")
	if appErr != nil {
		return ""
	}
	row, ok := rows[galgameID]
	if !ok {
		return ""
	}
	brief := client.CatalogItemToBrief(&row)
	return client.BriefName(&brief)
}

// firstNonEmpty returns the first non-blank argument, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ──────────────────────────────────────────
// GetDetail — GET /galgame/:gid
// ──────────────────────────────────────────

// GetDetail aggregates galgame metadata + local interaction stats into the
// full detail payload.
//
// token (Bearer access token from session, may be empty) is forwarded to
// galgame so its visibility filter sees the caller's identity — the
// submitter of a pending draft can view their own row, an authenticated
// user can see VNDB-source drafts, etc. Anonymous viewers get the same
// behavior as before (status=0 only).
//
// NSFW is NOT gated here — galgame's /galgame/:gid default is "不过滤"
// (docs/galgame_wiki/00-handbook §16.2: direct URL access is "有意为之").
// We deliberately let the response carry contentLimit through to the FE
// and let the FE decide UX: anonymous + SFW-cookie users see a "click
// to confirm" interstitial, logged-in OR NSFW-cookie-on users see the
// page directly. This trades a tiny SSR leak (mitigated by FE
// `useKunDisableSeo`) for a much better UX on shared NSFW links.
//
// The ENTRY is ungated; its adult TAG CHIPS are not. isSFW gates those at the
// bottom of this method — the one part of the payload whose leak has no UX
// argument behind it. See withoutSexualTags.
func (s *GalgameService) GetDetail(
	ctx context.Context,
	galgameID, currentUserID int,
	token string,
	isSFW bool,
) (*dto.GalgameDetail, *errors.AppError) {
	// content_limit=all (permissive): /galgame/:gid is "不过滤" (§16.2 direct URL
	// access is 有意为之) — the FE decides the NSFW interstitial. /v1 serves only
	// published rows, so a banned (status=1) or unknown id comes back as a
	// not-found AppError (the bridge's "该 Galgame 已被封禁" message is subsumed by
	// the standard not-found — /v1 never returns a banned row).
	d, found, appErr := s.galgameClient.CatalogWorkDetail(ctx, galgameID)
	if appErr != nil {
		return nil, appErr
	}
	if !found {
		return nil, errors.ErrNotFound("未找到该 Galgame")
	}
	g := client.CatalogDetailToFull(d, galgameID)
	// The maker's 官网 rides on the LABEL record, not on the work's attribution
	// edge, so it takes a second (memoized) lookup. Without it the 制作方 block
	// says 暂无官网 for every maker, homepage or not.
	s.galgameClient.HydrateOfficialLinks(ctx, &g)

	// Async view bump (don't block the response).
	go s.galgameRepo.IncrementView(galgameID)

	local := s.galgameRepo.FindLocal(galgameID)
	isLiked, isFavorited := s.interactionRepo.UserInteraction(currentUserID, galgameID)

	platforms, languages, types := s.resourceMetaRepo.FindResourceMetaByGalgame(galgameID)

	ratings := s.buildDetailRatings(ctx, galgameID, currentUserID, g)

	// The catalog carries no submitter (doc 106 R2), so the author comes from the
	// forum's own frozen snapshot — the same lane the edit permission check uses,
	// and status-blind like it.
	if owner := s.ownerOf(galgameID); owner > 0 {
		g.UserID = owner
	}
	users := s.hydrateDetailUsers(ctx, g)
	detail := galgameDetailFromNextMoe(g, users)
	// 正版购买 (DLsite affiliate) — empty unless this galgame carries a DLsite work
	// number AND the affiliate template is configured.
	if purchase := dlsite.LinkFor(s.dlsiteLinkTemplate, g.ID, g.Refs["dlsite"]); purchase != "" {
		detail.DlsitePurchaseURL = purchase
		detail.DlsiteCouponURL = s.dlsiteCouponURL
	}
	detail.View = local.View
	detail.LikeCount = local.LikeCount
	detail.FavoriteCount = local.FavoriteCount
	detail.ResourcePublishBanned = local.ResourcePublishBanned
	// No local row ⇒ a galgame-catalogue game the forum has never ingested. The FE
	// then shows a 未收录 notice + hides the (always-0) view count, but keeps the
	// upload/rate/comment CTAs that create the local row on first use.
	detail.IsOnForum = local.ID != 0
	detail.IsLiked = isLiked
	detail.IsFavorited = isFavorited
	detail.Platform = platforms
	detail.Language = languages
	detail.Type = types
	detail.Ratings = ratings
	// The GAME is not gated (see the note above — a direct URL is 有意为之), but
	// its adult TAG chips are: `isSFW` arrived here unused, so the sexual
	// vocabulary shipped in the SSR/__NUXT__ payload of every entry, to
	// crawlers included. The FE keeps its own category toggle for NSFW-enabled
	// viewers, who still receive the whole set.
	if isSFW {
		detail.Tag = withoutSexualTags(detail.Tag)
	}
	return &detail, nil
}

// hydrateDetailUsers resolves the author + every contributor from OAuth into the
// uid-keyed users map galgameDetailFromNextMoe consumes (the /v1 detail carries
// no users map — the bridge's map was itself an OAuth resolution, so this is the
// same source).
func (s *GalgameService) hydrateDetailUsers(ctx context.Context, g dto.NextMoeGalgameDetailFull) map[string]dto.NextMoeUser {
	uids := make([]int, 0, len(g.Contributor)+1)
	uids = append(uids, g.UserID)
	for _, c := range g.Contributor {
		uids = append(uids, c.UserID)
	}
	umap := s.userClient.Hydrate(ctx, uids)
	users := make(map[string]dto.NextMoeUser, len(umap))
	for id, u := range umap {
		users[strconv.Itoa(id)] = dto.NextMoeUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
	}
	return users
}

// buildDetailRatings assembles the ratings list with user resolution and liked flag.
func (s *GalgameService) buildDetailRatings(
	ctx context.Context,
	galgameID, currentUserID int,
	g dto.NextMoeGalgameDetailFull,
) []dto.GalgameDetailRating {
	rows := s.detailRatingRepo.FindRatingsByGalgame(galgameID)
	if len(rows) == 0 {
		return []dto.GalgameDetailRating{}
	}

	userIDs := make([]int, len(rows))
	ratingIDs := make([]int, len(rows))
	for i, r := range rows {
		userIDs[i] = r.UserID
		ratingIDs[i] = r.ID
	}
	userMap := s.userClient.Hydrate(ctx, userIDs)
	likedSet := s.detailRatingRepo.FindLikedRatingIDs(currentUserID, ratingIDs)

	out := make([]dto.GalgameDetailRating, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		out = append(out, detailRatingFromRow(r, u, likedSet[r.ID], galgameID, g))
	}
	return out
}

// ──────────────────────────────────────────
// GetList — GET /galgame
// ──────────────────────────────────────────

func (s *GalgameService) GetList(
	ctx context.Context,
	req *dto.GalgameListRequest,
	isSFW bool,
) (*dto.GalgameListPage, *errors.AppError) {
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Resolve the release-date filter (galgame §17 "YYYY"/"YYYY-MM") to
	// inclusive date boundaries. Malformed input is a client error, not
	// a silently-ignored param.
	releasedFrom, err := utils.ParseReleaseLowerBound(req.ReleasedFrom)
	if err != nil {
		return nil, errors.ErrBadRequest(err.Error())
	}
	releasedTo, err := utils.ParseReleaseUpperBound(req.ReleasedTo)
	if err != nil {
		return nil, errors.ErrBadRequest(err.Error())
	}
	releasedMonths, err := utils.ParseMonthSet(req.ReleasedMonths)
	if err != nil {
		return nil, errors.ErrBadRequest(err.Error())
	}

	filter := model.GalgameListFilter{
		Type:                 req.Type,
		Language:             req.Language,
		Platform:             req.Platform,
		GameType:             req.GameType,
		SortField:            req.SortField,
		SortOrder:            sortOrder,
		IncludeProviders:     splitCSV(req.IncludeProviders),
		ExcludeOnlyProviders: splitCSV(req.ExcludeOnlyProviders),
		ReleasedFrom:         releasedFrom,
		ReleasedTo:           releasedTo,
		ReleasedMonths:       releasedMonths,
		MinRatingCount:       req.MinRatingCount,
		MinRating:            req.MinRating,
		ShowNoResource:       req.ShowNoResource,
		Page:                 req.Page,
		Limit:                req.Limit,
	}

	return s.hydrateListCards(ctx, filter, isSFW)
}

// hydrateListCards runs the shared "filter → ids → hydrate → cards" flow used by
// BOTH the global /galgame list AND the galgame-entity detail pages (tag/official/
// engine, which set filter.RestrictIDs = the galgame member ids). All filtering /
// sorting / pagination is local (list_repo over galgame_resource); hydration
// pulls galgame metadata + OAuth users + local stats/ratings/resource-meta. Keeping
// this in one place is why the entity pages add zero duplicated filter logic.
func (s *GalgameService) hydrateListCards(
	ctx context.Context,
	filter model.GalgameListFilter,
	isSFW bool,
) (*dto.GalgameListPage, *errors.AppError) {
	// Note: `total` from listRepo is the count of kungal-known galgames (stats
	// rows) and can over-report when galgame drops NSFW briefs in SFW mode; an exact
	// total requires the public list to source from galgame's /galgame, not kungal's
	// local stats — out of scope here.
	ids, total := s.listRepo.ListIDs(filter)
	if len(ids) == 0 {
		return &dto.GalgameListPage{Galgames: []dto.GalgameListCard{}, Total: total}, nil
	}
	cards, appErr := s.HydrateCardsByIDs(ctx, ids, isSFW)
	if appErr != nil {
		return nil, appErr
	}
	return &dto.GalgameListPage{Galgames: cards, Total: total}, nil
}

// HydrateCardsByIDs turns an ORDERED galgame id list into list cards, fusing
// galgame metadata + OAuth users + local stats/ratings/resource-meta. The output
// preserves the input order and drops ids the galgame filtered out (NSFW miss /
// deleted). Shared by the global list, the galgame-entity detail pages, AND the
// collection detail (收藏夹) so none of them duplicate the hydration.
//
// SFW gating is delegated to galgame via content_limit per
// docs/galgame_wiki/00-handbook §16 — no service-layer post-filter (would
// violate "galgame is the only NSFW SoT"). A galgame-batch error is surfaced, not
// silently degraded to a blank list.
func (s *GalgameService) HydrateCardsByIDs(
	ctx context.Context,
	ids []int,
	isSFW bool,
) ([]dto.GalgameListCard, *errors.AppError) {
	if len(ids) == 0 {
		return []dto.GalgameListCard{}, nil
	}

	briefMap, appErr := s.galgameClient.GetBatchPublic(ctx, ids, isSFW)
	if appErr != nil {
		return nil, appErr
	}

	// Local stats batch — also the source of the author chip: the wiki-era
	// creator is frozen on the local row (migration 066) because the catalog
	// face carries no submitter (doc 106 R2 ②).
	localMap := s.galgameRepo.FindLocalBatch(ids)

	userMap := s.userClient.Hydrate(ctx, frozenCreatorIDs(ids, localMap))

	// Bayesian display rating per card (same formula as the rating sort).
	ratingMap := s.listRepo.BayesianRatings(ids)

	// Platform/language aggregation
	metaRows := s.resourceMetaRepo.FindResourceMetaBatch(ids)
	platformMap, languageMap := groupResourceMeta(metaRows)

	cards := make([]dto.GalgameListCard, 0, len(ids))
	for _, id := range ids {
		b, ok := briefMap[id]
		if !ok {
			continue
		}
		cards = append(cards, dto.GalgameListCard{
			ID: id,
			Name: dto.KunLanguage{
				EnUs: b.NameEnUs, JaJp: b.NameJaJp,
				ZhCn: b.NameZhCn, ZhTw: b.NameZhTw,
			},
			Banner:       b.Banner,
			User:         frozenCreatorBrief(localMap[id], userMap),
			ContentLimit: b.ContentLimit,
			View:         localMap[id].View,
			LikeCount:    localMap[id].LikeCount,
			Rating:       ratingMap[id].Score,
			RatingCount:  ratingMap[id].Count,
			// kungal's own list: the displayed "最近更新" comes from the LOCAL
			// resource_update_time (the sort key), NOT the galgame's (which never
			// tracks kungal resource activity) — so order and label agree.
			ResourceUpdateTime:       localMap[id].ResourceUpdateTime.Format(time.RFC3339),
			ReleaseDate:              b.ReleaseDate,
			ReleaseDateTBA:           b.ReleaseDateTBA,
			EffectiveBannerHash:      b.EffectiveBannerHash,
			EffectiveBannerURL:       b.EffectiveBannerURL,
			EffectiveBannerWidth:     b.EffectiveBannerWidth,
			EffectiveBannerHeight:    b.EffectiveBannerHeight,
			EffectiveBannerThumbhash: b.EffectiveBannerThumbhash,
			Platform:                 emptyStrSliceIfNil(platformMap[id]),
			Language:                 emptyStrSliceIfNil(languageMap[id]),
		})
	}
	return cards, nil
}
