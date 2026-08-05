package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/trust/gate"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RatingService struct {
	ratingRepo    *repository.RatingRepository
	galgameClient *client.GalgameClient
	userClient    *userclient.Client
	check         *gate.CheckService
	scan          *gate.ScanService
	helpers       InteractionHelpers
}

func NewRatingService(
	ratingRepo *repository.RatingRepository,
	galgameClient *client.GalgameClient,
	userClient *userclient.Client,
	check *gate.CheckService,
	scan *gate.ScanService,
) *RatingService {
	return &RatingService{
		ratingRepo:    ratingRepo,
		galgameClient: galgameClient,
		userClient:    userClient,
		check:         check,
		scan:          scan,
	}
}

// ratingReward applies the documented thresholds (architecture-patterns spec):
// summary < 233 → +3, < 666 → +5, ≥ 666 → +10.
func ratingReward(summaryLen int) int {
	switch {
	case summaryLen >= constants.RatingLenThresholdHigh:
		return constants.RatingRewardHigh
	case summaryLen >= constants.RatingLenThresholdMedium:
		return constants.RatingRewardMedium
	default:
		return constants.RatingRewardLow
	}
}

// ──────────────────────────────────────────
// GetAllRatings — GET /galgame-rating/all
// ──────────────────────────────────────────

// GetAllRatings returns the public rating list. SFW filter is applied
// at the service layer against each rating's galgame brief (content_limit
// lives only on galgame, not on the local galgame_rating row), so `total`
// over-reports in SFW mode — same SEO-safe trade-off as
// galgame_service.GetList.
func (s *RatingService) GetAllRatings(
	ctx context.Context,
	req *dto.RatingListRequest,
	isSFW bool,
) (*dto.RatingListPage, *errors.AppError) {
	// Normalise sort order
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	rows, total := s.ratingRepo.ListPaginated(model.RatingFilter{
		SpoilerLevel: req.SpoilerLevel,
		PlayStatus:   req.PlayStatus,
		GalgameType:  req.GalgameType,
		SortField:    req.SortField,
		SortOrder:    sortOrder,
		Page:         req.Page,
		Limit:        req.Limit,
	})

	// Batch resolve users and galgames
	userIDs := make([]int, len(rows))
	galgameIDs := make([]int, len(rows))
	for i, r := range rows {
		userIDs[i] = r.UserID
		galgameIDs[i] = r.GalgameID
	}
	userMap := s.userClient.Hydrate(ctx, userIDs)
	// Galgame-side SFW filter (content_limit) per
	// docs/galgame_wiki/00-handbook §16. Rows whose galgame is filtered
	// come back as "no brief returned" and get dropped below.
	briefMap := s.fetchGalgameBriefsPublic(ctx, galgameIDs, isSFW)

	cards := make([]dto.RatingCard, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		b, hasBrief := briefMap[r.GalgameID]
		if !hasBrief {
			continue
		}
		cards = append(cards, ratingRowToCard(r, u, b))
	}

	return &dto.RatingListPage{RatingData: cards, Total: total}, nil
}

// ──────────────────────────────────────────
// GetRatingDetail — GET /galgame-rating/:id
// ──────────────────────────────────────────

// GetRatingDetail — NSFW gate intentionally absent here; mirrors the
// galgame/topic detail policy. FE decides UX based on the embedded
// galgame.contentLimit and the caller's cookie+login state.
func (s *RatingService) GetRatingDetail(
	ctx context.Context,
	ratingID, currentUserID int,
) (*dto.RatingDetail, *errors.AppError) {
	row, ok := s.ratingRepo.FindByID(ratingID)
	if !ok {
		return nil, errors.ErrNotFound("评分不存在")
	}

	// A banned author's rating is hidden even by direct link — 404 before
	// counting a view. (Cached; the batch hydrate below reuses it.)
	if author, _, _ := s.userClient.User(ctx, row.UserID); !userclient.IsRenderable(author) {
		return nil, errors.ErrNotFound("评分不存在")
	}

	// Fire-and-forget view increment; reflect it in response too.
	s.ratingRepo.IncrementView(ratingID)
	row.View++

	// Liked users
	likerIDs := s.ratingRepo.FindLikerIDs(ratingID)
	isLiked := containsInt(likerIDs, currentUserID)

	// Author + likers — collect uids and hydrate in one batch. The rating
	// comment embed was dropped in charter step 06a: the comment area now reads
	// from the community primitive via /galgame-rating/:id/comments, and the FE
	// no longer consumes a `comments` field on the detail response.
	uidSet := map[int]struct{}{row.UserID: {}}
	for _, id := range likerIDs {
		uidSet[id] = struct{}{}
	}
	uids := make([]int, 0, len(uidSet))
	for id := range uidSet {
		uids = append(uids, id)
	}
	userMap := s.userClient.Hydrate(ctx, uids)

	// Galgame detail from galgame + (when present) its parent series.
	galgame := s.buildRatingGalgame(ctx, row.GalgameID)

	// Author projection of liked users (preserve liker order).
	authorBriefs := make([]dto.UserBrief, 0, len(likerIDs))
	for _, id := range likerIDs {
		u := userMap[id]
		if !userclient.IsRenderable(u) {
			continue
		}
		authorBriefs = append(authorBriefs, userBriefToDTO(u))
	}

	detail := &dto.RatingDetail{
		ID:           row.ID,
		User:         userBriefToDTO(userMap[row.UserID]),
		Recommend:    row.Recommend,
		Overall:      row.Overall,
		View:         row.View,
		GalgameType:  rawJSON(row.GalgameType),
		PlayStatus:   row.PlayStatus,
		ShortSummary: row.ShortSummary,
		SpoilerLevel: row.SpoilerLevel,
		RatingScores: rowToScores(row),
		LikeCount:    len(likerIDs),
		IsLiked:      isLiked,
		LikedUsers:   authorBriefs,
		Created:      row.Created,
		Updated:      row.Updated,
		Galgame:      galgame,
	}
	return detail, nil
}

// ──────────────────────────────────────────
// CreateRating — POST /galgame-rating
// One rating per user per galgame. Reward tier follows summary length.
// ──────────────────────────────────────────

func (s *RatingService) CreateRating(
	ctx context.Context,
	userID int,
	req *dto.CreateRatingRequest,
) (*dto.CreatedRating, *errors.AppError) {
	if s.ratingRepo.ExistsByUserGalgame(req.GalgameID, userID) {
		return nil, errors.ErrBadRequest("您已经发布过该 Galgame 的评分了")
	}

	// Synchronous word-list gate BEFORE the tx (trust wave 2): the rating's free
	// text is its short_summary. deny blocks (nothing persisted); hold publishes+
	// logs; fail-open on any error/timeout.
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, req.ShortSummary, &authorID)
	if decision == gate.DecisionDeny {
		return nil, gate.ErrContentBlocked()
	}

	galgameTypeJSON, _ := json.Marshal(req.GalgameType)
	rating := &model.GalgameRating{
		Recommend:    req.Recommend,
		Overall:      req.Overall,
		GalgameType:  galgameTypeJSON,
		PlayStatus:   req.PlayStatus,
		ShortSummary: req.ShortSummary,
		SpoilerLevel: req.SpoilerLevel,
		Art:          req.Art, Story: req.Story, Music: req.Music,
		Character: req.Character, Route: req.Route, System: req.System,
		Voice: req.Voice, ReplayValue: req.ReplayValue,
		GalgameID: req.GalgameID, UserID: userID,
	}
	reward := ratingReward(len(req.ShortSummary))

	txErr := s.ratingRepo.DB().Transaction(func(tx *gorm.DB) error {
		// Lazy-create the local stub before the rating INSERT. galgame_rating
		// has an ON DELETE RESTRICT FK to the local galgame stub table, which
		// is sparse (only created on interactions); rating a galgame nobody
		// has otherwise touched would FK-violate → opaque 500. Matches the
		// comment / resource / like / favorite paths.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&model.GalgameLocal{ID: req.GalgameID}).Error; err != nil {
			return err
		}
		if err := s.ratingRepo.Create(tx, rating); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, userID, reward,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame_rating", rating.ID))
		return nil
	})
	if txErr != nil {
		return nil, errors.ErrInternal("创建评分失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindGalgameRating, "subject_id", rating.ID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindGalgameRating, strconv.Itoa(rating.ID), req.ShortSummary, int64(userID))

	user, _, _ := s.userClient.User(ctx, userID)
	briefMap := s.fetchGalgameBriefs(ctx, []int{req.GalgameID})

	return &dto.CreatedRating{
		ID:           rating.ID,
		User:         userBriefToDTO(user),
		Recommend:    rating.Recommend,
		Overall:      rating.Overall,
		View:         rating.View,
		GalgameType:  rawJSON(string(rating.GalgameType)),
		PlayStatus:   rating.PlayStatus,
		ShortSummary: rating.ShortSummary,
		SpoilerLevel: rating.SpoilerLevel,
		RatingScores: dto.RatingScores{
			Art: rating.Art, Story: rating.Story, Music: rating.Music,
			Character: rating.Character, Route: rating.Route, System: rating.System,
			Voice: rating.Voice, ReplayValue: rating.ReplayValue,
		},
		LikeCount: 0, IsLiked: false,
		Created: rating.CreatedAt.Format(time.RFC3339),
		Updated: rating.UpdatedAt.Format(time.RFC3339),
		Galgame: dto.RatingGalgameBrief{
			ID:           req.GalgameID,
			ContentLimit: briefMap[req.GalgameID].ContentLimit,
			Name: dto.KunLanguage{
				EnUs: briefMap[req.GalgameID].NameEnUs,
				JaJp: briefMap[req.GalgameID].NameJaJp,
				ZhCn: briefMap[req.GalgameID].NameZhCn,
				ZhTw: briefMap[req.GalgameID].NameZhTw,
			},
		},
	}, nil
}

// ──────────────────────────────────────────
// UpdateRating — PUT /galgame-rating/:id
// Author-only. Adjusts moemoepoint by the reward-tier delta when the summary
// length crosses a threshold.
// ──────────────────────────────────────────

func (s *RatingService) UpdateRating(
	ctx context.Context,
	userID int,
	req *dto.UpdateRatingRequest,
) *errors.AppError {
	rating, err := s.ratingRepo.FindRatingForWrite(req.GalgameRatingID)
	if err != nil {
		return errors.ErrNotFound("评分不存在")
	}
	if rating.UserID != userID {
		return errors.ErrForbidden("您无权限修改他人评分")
	}

	// Synchronous word-list gate BEFORE the tx (deny blocks, hold publishes+logs,
	// fail-open). author_id is the content author (rating.UserID).
	authorID := int64(rating.UserID)
	decision, matched := s.check.Decision(ctx, req.ShortSummary, &authorID)
	if decision == gate.DecisionDeny {
		return gate.ErrContentBlocked()
	}

	pointDiff := ratingReward(len(req.ShortSummary)) - ratingReward(len(rating.ShortSummary))
	galgameTypeJSON, _ := json.Marshal(req.GalgameType)
	fields := map[string]any{
		"recommend":     req.Recommend,
		"overall":       req.Overall,
		"galgame_type":  galgameTypeJSON,
		"play_status":   req.PlayStatus,
		"short_summary": req.ShortSummary,
		"spoiler_level": req.SpoilerLevel,
		"art":           req.Art, "story": req.Story, "music": req.Music,
		"character": req.Character, "route": req.Route, "system": req.System,
		"voice": req.Voice, "replay_value": req.ReplayValue,
	}

	txErr := s.ratingRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.ratingRepo.Update(tx, req.GalgameRatingID, fields); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, userID, pointDiff,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame_rating", req.GalgameRatingID))
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("更新评分失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindGalgameRating, "subject_id", req.GalgameRatingID, "author_id", rating.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindGalgameRating, strconv.Itoa(req.GalgameRatingID), req.ShortSummary, int64(rating.UserID))
	return nil
}

// ──────────────────────────────────────────
// DeleteRating — DELETE /galgame-rating/:id
// Allowed by author, by galgame owner, or admin. Refunds the original reward.
// ──────────────────────────────────────────

func (s *RatingService) DeleteRating(
	userID int, canModerate bool, ratingID int,
) *errors.AppError {
	rating, err := s.ratingRepo.FindRatingForWrite(ratingID)
	if err != nil {
		return errors.ErrNotFound("未找到评分")
	}
	galgameOwner := s.ratingRepo.FindGalgameOwner(rating.GalgameID)
	if rating.UserID != userID && galgameOwner != userID && !canModerate {
		return errors.ErrForbidden("没有删除该评分的权限")
	}

	refund := -ratingReward(len(rating.ShortSummary))
	txErr := s.ratingRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.ratingRepo.DeleteByID(tx, ratingID); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, rating.UserID, refund,
			moemoepoint.ReasonContentRemoved, moemoepoint.Ref("galgame_rating", ratingID))
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("删除评分失败")
	}
	return nil
}

// ──────────────────────────────────────────
// ToggleRatingLike — PUT /galgame-rating/:id/like
// Self-like is rejected. Like grants +1 to the rating author; unlike reverses.
// Sends a deduped 'liked' notification with the summary preview as content.
// ──────────────────────────────────────────

func (s *RatingService) ToggleRatingLike(
	userID int,
	req *dto.ToggleRatingLikeRequest,
) *errors.AppError {
	rating, err := s.ratingRepo.FindRatingForWrite(req.GalgameRatingID)
	if err != nil {
		return errors.ErrNotFound("评分不存在")
	}
	if rating.UserID == userID {
		return errors.ErrBadRequest("不能给自己的评分点赞")
	}

	preview := truncate(rating.ShortSummary, constants.TextPreviewLength)
	txErr := s.ratingRepo.DB().Transaction(func(tx *gorm.DB) error {
		existing, has := s.ratingRepo.FindLike(tx, req.GalgameRatingID, userID)
		var delta int
		if has {
			if err := s.ratingRepo.DeleteLike(tx, existing); err != nil {
				return err
			}
			delta = -1
		} else {
			if err := s.ratingRepo.CreateLike(tx, req.GalgameRatingID, userID); err != nil {
				return err
			}
			delta = 1
		}
		if err := s.ratingRepo.AdjustLikeCount(tx, req.GalgameRatingID, delta); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, rating.UserID, delta,
			moemoepoint.ReasonLiked, moemoepoint.Ref("galgame_rating", req.GalgameRatingID))
		s.helpers.CreateGalgameMessageWithContent(
			tx, userID, rating.UserID, "liked", preview, rating.GalgameID,
		)
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

// The rating comment area moved to the infra community primitive (charter step
// 06a): the /galgame-rating/:id/comment CRUD was retired, its DTOs + repo methods
// removed, and the galgame_rating_comment table dropped (migration 060). Comments
// are now served by the shared resource-comment BFF on the `/comments` routes.

// ──────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────

func (s *RatingService) fetchGalgameBriefs(
	ctx context.Context,
	galgameIDs []int,
) map[int]client.GalgameBrief {
	if len(galgameIDs) == 0 {
		return map[int]client.GalgameBrief{}
	}
	m, _ := s.galgameClient.GetBatch(ctx, galgameIDs)
	if m == nil {
		return map[int]client.GalgameBrief{}
	}
	return m
}

// fetchGalgameBriefsPublic is the SFW-aware variant — for public list paths
// that must honour content_limit per docs/galgame_wiki/00-handbook §16.
func (s *RatingService) fetchGalgameBriefsPublic(
	ctx context.Context,
	galgameIDs []int,
	isSFW bool,
) map[int]client.GalgameBrief {
	if len(galgameIDs) == 0 {
		return map[int]client.GalgameBrief{}
	}
	m, _ := s.galgameClient.GetBatchPublic(ctx, galgameIDs, isSFW)
	if m == nil {
		return map[int]client.GalgameBrief{}
	}
	return m
}

// buildRatingGalgame fetches the rated work's catalog aggregate and fuses in
// the local rating stats.
//
// The "所属系列" panel and the Review JSON-LD `isPartOf` clause are gone with
// the wiki series vocabulary (P3): 146 wiki series, only 6 with any catalog
// counterpart, so there was nothing to re-anchor them to.
func (s *RatingService) buildRatingGalgame(
	ctx context.Context,
	galgameID int,
) dto.RatingGalgameDetail {
	summary := dto.RatingGalgameDetail{
		ID:       galgameID,
		Official: []dto.RatingOfficial{},
	}
	// content_limit=all (permissive): the rating page doesn't gate NSFW here (FE
	// confirm card). Best-effort — an unreachable / not-found galgame leaves the
	// summary at its zero-ish defaults + local rating stats below.
	if d, found, appErr := s.galgameClient.CatalogWorkDetail(ctx, galgameID); appErr == nil && found {
		g := client.CatalogDetailToFull(d, galgameID)
		// Same 制作方 block as the detail page, so the same 官网 hydration.
		s.galgameClient.HydrateOfficialLinks(ctx, &g)
		summary.ID = g.ID
		summary.Banner = g.Banner
		summary.EffectiveBannerHash = g.EffectiveBannerHash
		summary.EffectiveBannerURL = g.EffectiveBannerURL
		summary.EffectiveBannerWidth = g.EffectiveBannerWidth
		summary.EffectiveBannerHeight = g.EffectiveBannerHeight
		summary.EffectiveBannerThumbhash = g.EffectiveBannerThumbhash
		summary.ContentLimit = g.ContentLimit
		summary.AgeLimit = g.AgeLimit
		summary.OriginalLanguage = g.OriginalLanguage
		summary.Name = dto.KunLanguage{
			EnUs: g.NameEnUs, JaJp: g.NameJaJp,
			ZhCn: g.NameZhCn, ZhTw: g.NameZhTw,
		}
		summary.Official = nextMoeOfficialsToDTO(g.Official)
	}

	sum, count := s.ratingRepo.GalgameRatingStats(galgameID)
	summary.Rating = sum
	summary.RatingCount = count
	return summary
}
