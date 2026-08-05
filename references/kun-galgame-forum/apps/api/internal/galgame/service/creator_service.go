package service

import (
	"context"
	"encoding/json"

	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/role"
	"kun-galgame-api/pkg/userclient"
)

// Forum creator-eligibility thresholds. This is the forum's OWN policy — change
// freely here; OAuth and the galgame are untouched (the role + queue live in
// OAuth, the contribution data in the galgame). A user may apply if ANY criterion
// is met. Cross-service contract owned by OAuth (not yet mirrored under docs/).
const (
	creatorMinMergedPRs   = 5    // 合并的 PR 数（数据源:galgame /user/:id/stats）
	creatorMinGalgames    = 10   // 已发布 galgame 数（数据源:galgame）
	creatorMinReviews     = 5    // 简评(≥100 字)数（数据源:本论坛 galgame_rating）
	creatorReviewMinLen   = 100  // 简评字数门槛
	creatorMinMoemoepoint = 2000 // 萌萌点（数据源:OAuth 权威余额，单一来源）
	creatorSource         = "forum"
)

// CreatorEligibility is the forum-side eligibility snapshot (current counts vs
// thresholds) — drives the "离创作者还差…" UI and gates Apply.
type CreatorEligibility struct {
	Eligible          bool  `json:"eligible"`
	MergedPRs         int64 `json:"merged_prs"`
	GalgamesPublished int64 `json:"galgames_published"`
	Reviews100        int64 `json:"reviews_100"`
	Moemoepoint       int64 `json:"moemoepoint"`
	NeedMergedPRs     int   `json:"need_merged_prs"`
	NeedGalgames      int   `json:"need_galgames"`
	NeedReviews       int   `json:"need_reviews"`
	NeedMoemoepoint   int   `json:"need_moemoepoint"`
}

// CreatorService computes forum-side creator eligibility and proxies the
// application to the central OAuth queue. The forum owns the POLICY; OAuth owns
// the queue + admin review + role grant.
type CreatorService struct {
	ratingRepo *repository.RatingRepository
	stats      *GalgameUserStatsService
	userClient *userclient.Client
}

func NewCreatorService(ratingRepo *repository.RatingRepository, stats *GalgameUserStatsService, userClient *userclient.Client) *CreatorService {
	return &CreatorService{ratingRepo: ratingRepo, stats: stats, userClient: userClient}
}

func (s *CreatorService) eligibility(ctx context.Context, userID int) (*CreatorEligibility, *errors.AppError) {
	// The two galgame criteria are derived from the registry now: accepted edits
	// are merged proposals, published entries are live claims. Both degrade to 0
	// rather than failing the snapshot — eligibility is a disjunction, so a
	// missing criterion must not deny the ones that are present.
	stats := s.stats.Stats(ctx, int64(userID))
	reviews, rErr := s.ratingRepo.CountReviewsWithMinLength(userID, creatorReviewMinLen)
	if rErr != nil {
		return nil, errors.ErrInternal("统计简评失败")
	}
	// Authoritative OAuth balance (single-sourced, C3). A fetch miss degrades to
	// 0 — this is one of several OR criteria, so it shouldn't fail the whole
	// snapshot; the user can still qualify via PR / galgame / 简评.
	moe, _ := s.userClient.GetMoemoepoint(ctx, userID)
	e := &CreatorEligibility{
		MergedPRs:         stats.MergedEdits,
		GalgamesPublished: stats.Published,
		Reviews100:        reviews,
		Moemoepoint:       int64(moe),
		NeedMergedPRs:     creatorMinMergedPRs,
		NeedGalgames:      creatorMinGalgames,
		NeedReviews:       creatorMinReviews,
		NeedMoemoepoint:   creatorMinMoemoepoint,
	}
	e.Eligible = e.MergedPRs >= creatorMinMergedPRs ||
		e.GalgamesPublished >= creatorMinGalgames ||
		e.Reviews100 >= creatorMinReviews ||
		e.Moemoepoint >= creatorMinMoemoepoint
	return e, nil
}

// Status returns the user's eligibility snapshot, current OAuth application
// (nil if never applied), and whether they ALREADY hold the creator role.
//
// isCreator is the source of truth (the role), separate from the application:
// it covers an admin granting the role directly (no approved application) and
// the window after approval before the role cache refreshes. A user-lookup miss
// degrades to false — the apply flow still works (OAuth's 17001 guards a dup).
func (s *CreatorService) Status(ctx context.Context, userID int, token string) (*CreatorEligibility, *userclient.CreatorApplication, bool, *errors.AppError) {
	e, appErr := s.eligibility(ctx, userID)
	if appErr != nil {
		return nil, nil, false, appErr
	}
	app, err := s.userClient.GetMyCreatorApplication(ctx, token)
	if err != nil {
		return nil, nil, false, errors.ErrInternal("获取申请状态失败")
	}
	isCreator := false
	if u, ok, uErr := s.userClient.User(ctx, userID); ok && uErr == nil {
		isCreator = role.IsCreator(u.Roles)
	}
	return e, app, isCreator, nil
}

// Apply enforces the forum eligibility gate, then files the application on the
// central OAuth queue with evidence. OAuth's own guards (already-creator /
// one-pending / cooldown) surface via the returned message.
func (s *CreatorService) Apply(ctx context.Context, userID int, token, message string) (*userclient.CreatorApplication, *errors.AppError) {
	e, appErr := s.eligibility(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if !e.Eligible {
		return nil, errors.ErrForbidden("尚不满足创作者申请条件")
	}
	evidence, _ := json.Marshal(map[string]any{
		"merged_prs":         e.MergedPRs,
		"galgames_published": e.GalgamesPublished,
		"reviews_100":        e.Reviews100,
		"moemoepoint":        e.Moemoepoint,
	})
	app, err := s.userClient.CreateCreatorApplication(ctx, token, creatorSource, evidence, message)
	if err != nil {
		if oe, ok := err.(*userclient.OAuthError); ok {
			return nil, errors.ErrBadRequest(oe.Message)
		}
		return nil, errors.ErrInternal("提交申请失败")
	}
	return app, nil
}
