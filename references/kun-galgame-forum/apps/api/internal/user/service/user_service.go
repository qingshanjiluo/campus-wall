package service

import (
	"context"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"kun-galgame-api/internal/galgame/client"
	galgameService "kun-galgame-api/internal/galgame/service"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/user/dto"
	"kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/role"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
)

// UserService surfaces the kungal-side user pages: profile, status,
// floating card, and check-in. Identity (name / avatar / bio / status /
// roles) is fetched from OAuth via userclient. Per-site state
// (moemoepoint, daily counters) lives in kungal_user_state. Content
// counts (topic / reply / etc.) come from local aggregates.
type UserService struct {
	stateRepo     *repository.StateRepository
	userStatsRepo *repository.UserStatsRepository
	rdb           *redis.Client
	galgameClient *client.GalgameClient
	galgameStats  *galgameService.GalgameUserStatsService
	userClient    *userclient.Client
	community     *communityclient.Client
	// commentCache memoizes per-user community visible_posts for a few minutes so
	// the floating hover card (fired on every hover) doesn't hit the S2S face each
	// time. Best-effort: an S2S error serves the last good value or 0.
	commentCache *visiblePostsCache
}

func NewUserService(
	stateRepo *repository.StateRepository,
	userStatsRepo *repository.UserStatsRepository,
	rdb *redis.Client,
	galgameClient *client.GalgameClient,
	galgameStats *galgameService.GalgameUserStatsService,
	userClient *userclient.Client,
	community *communityclient.Client,
) *UserService {
	return &UserService{
		stateRepo:     stateRepo,
		userStatsRepo: userStatsRepo,
		rdb:           rdb,
		galgameClient: galgameClient,
		galgameStats:  galgameStats,
		userClient:    userClient,
		community:     community,
		commentCache:  newVisiblePostsCache(),
	}
}

// ──────────────────────────────────────────
// Profile
// ──────────────────────────────────────────

func (s *UserService) GetUserProfile(ctx context.Context, userID int) (*dto.UserProfileDetail, *errors.AppError) {
	u, ok, err := s.userClient.User(ctx, userID)
	if err != nil {
		return nil, errors.ErrInternal("查询用户信息失败")
	}
	if !ok {
		return nil, errors.ErrNotFound("未找到该用户")
	}
	// Banned users surface only id+name so callers can render a "已封禁"
	// placeholder; we don't leak avatar / bio / stats.
	if u.Status != 0 {
		return &dto.UserProfileDetail{ID: u.ID, Name: u.Name, Status: u.Status}, nil
	}

	stats, err := s.userStatsRepo.GetUserStats(userID)
	if err != nil {
		return nil, errors.ErrInternal("获取用户统计失败")
	}
	state, _ := s.stateRepo.FindByID(userID)
	moe := 0
	if state != nil {
		moe = state.Moemoepoint
	}

	profile := &dto.UserProfileDetail{
		ID:          u.ID,
		Name:        u.Name,
		Avatar:      u.Avatar,
		Roles:       u.Roles,
		Status:      u.Status,
		Moemoepoint: moe,
		Bio:         u.Bio,
	}
	// CreatedAt is the user's real OAuth registration time (the authoritative
	// "join date"), sourced from the OAuth brief. We deliberately do NOT use
	// kungal_user_state.created — that only marks "first seen on kungal":
	// blank for users who registered but never logged into the forum, and
	// wrong for those whose first forum login lagged their registration. Fall
	// back to the local state row only if OAuth somehow omitted created_at.
	if t, perr := time.Parse(time.RFC3339, u.CreatedAt); perr == nil {
		profile.CreatedAt = t
	} else if state != nil {
		profile.CreatedAt = state.CreatedAt
	}

	profile.Topic = stats.Topic
	profile.TopicPoll = stats.TopicPoll
	profile.ReplyCreated = stats.ReplyCreated
	profile.CommentCreated = stats.CommentCreated
	// galgame_comment now means "all community comment areas": sourced from the
	// primitive's visible_posts (charter step 06a), not the frozen table.
	profile.GalgameComment = s.communityVisiblePosts(ctx, userID)
	profile.GalgameRating = stats.GalgameRating
	profile.GalgameResource = stats.GalgameResource
	profile.GalgameToolset = stats.GalgameToolset
	profile.GalgameToolsetResource = stats.GalgameToolsetResource
	profile.Upvote = stats.Upvote
	profile.Like = stats.Like
	profile.Dislike = stats.Dislike
	profile.DailyTopicCount = stats.DailyTopicCount

	// Derived from the registry rather than counted in a wiki table: published =
	// this user's live claims, contributed = the DISTINCT entries their merged
	// edits touched. Both degrade to 0 on an unreachable face; a profile page
	// has other reasons to render.
	galgameStats := s.galgameStats.Stats(ctx, int64(userID))
	profile.Galgame = galgameStats.Published
	profile.DailyGalgameCount = int64(galgameStats.PublishedToday)
	profile.ContributeGalgame = int64(galgameStats.Contributed)

	return profile, nil
}

// ──────────────────────────────────────────
// Check-in / status
// ──────────────────────────────────────────

func (s *UserService) CheckIn(ctx context.Context, userID int) (int, *errors.AppError) {
	// Atomic once-per-day gate: CheckIn flips daily_check_in only when it's 0
	// (reset at calendar midnight by the daily cron). No read-then-write race
	// and no rate limiter — a repeat attempt today applies nothing → "已签到".
	applied, err := s.stateRepo.CheckIn(userID)
	if err != nil {
		return 0, errors.ErrInternal("签到失败")
	}
	if !applied {
		return 0, errors.ErrBadRequest("您今天已经签到过了")
	}

	// Reward via OAuth (single source). The per-user-per-day idempotency key
	// makes it replay-safe; the daily flag is already set so a failed award
	// doesn't let the user re-check. Awarder mirrors the authoritative balance
	// into the local cache. points==0 is a no-op (Awarder skips zero delta).
	points := rand.IntN(8) // 0-7
	moemoepoint.Award(userID, points, moemoepoint.ReasonDailyCheckin, "",
		moemoepoint.Key("checkin", strconv.Itoa(userID), time.Now().Format("2006-01-02")))
	return points, nil
}

// GetMoemoepointLog returns one cursor-paginated page of the user's unified
// moemoepoint ledger, proxied from OAuth (the single source). The ledger spans
// ALL sites (a like earned on moyu shows up here too), since the balance is
// unified. Scoped to the caller — the handler passes the session user's id.
func (s *UserService) GetMoemoepointLog(
	ctx context.Context,
	userID, limit, beforeID int,
	reason string,
) (userclient.MoemoepointLogPage, *errors.AppError) {
	page, err := s.userClient.MoemoepointLog(ctx, userID, limit, beforeID, reason)
	if err != nil {
		return userclient.MoemoepointLogPage{}, errors.ErrInternal("获取萌萌点明细失败")
	}
	return page, nil
}

func (s *UserService) GetUserStatus(ctx context.Context, userID int) (*dto.UserStatusResponse, *errors.AppError) {
	// A freshly-registered user may not have a state row yet (the
	// callback flow creates it lazily). Treat that case as zero-state
	// rather than 404 — the FE Nav.vue otherwise silently drops the
	// moemoepoint chip + check-in badge with no fallback, so brand-new
	// users see a half-broken nav until they click "签到" the first
	// time (which is itself gated behind reading moemoepoint…).
	moe := 0
	isCheckIn := false
	var uploadBytes int64
	var mutedTypes []string
	if state, err := s.stateRepo.FindByID(userID); err == nil && state != nil {
		moe = state.Moemoepoint
		isCheckIn = state.DailyCheckIn == 1
		uploadBytes = state.DailyToolsetUploadBytes
		mutedTypes = state.MutedNotificationTypes
	}

	// Notification preferences (migration 053): muted categories don't drive the
	// red dot. localMuted drops those message.type rows from the unread count;
	// muting the "chat" stream zeroes its unread contribution. Official system
	// broadcasts are non-mutable, so they always count. The rows still exist —
	// this only governs has_new_message.
	localMuted, chatMuted := msgService.SplitMuted(mutedTypes)

	unreadMessage, _ := s.userStatsRepo.CountUnreadMessages(userID, localMuted)
	unreadSystem, _ := s.userStatsRepo.CountUnreadSystemMessages(userID)
	var unreadChat int64
	if !chatMuted {
		unreadChat, _ = s.userStatsRepo.CountUnreadChatMessages(userID)
	}

	// Live creator-role check (cached ~10min) so the avatar menu can hide the
	// "创作者申请" entry once the role is held. A lookup miss degrades to false.
	isCreator := false
	if u, ok, uErr := s.userClient.User(ctx, userID); ok && uErr == nil {
		isCreator = role.IsCreator(u.Roles)
	}

	return &dto.UserStatusResponse{
		Moemoepoints:            moe,
		IsCheckIn:               isCheckIn,
		HasNewMessage:           (unreadMessage + unreadSystem + unreadChat) > 0,
		DailyToolsetUploadBytes: uploadBytes,
		IsCreator:               isCreator,
	}, nil
}

// GetNotificationPreferences returns the caller's muted notification-category
// set (migration 053). A missing state row (brand-new user) → empty set.
func (s *UserService) GetNotificationPreferences(userID int) (*dto.NotificationPreferenceResponse, *errors.AppError) {
	muted := []string{}
	if state, err := s.stateRepo.FindByID(userID); err == nil && state != nil && len(state.MutedNotificationTypes) > 0 {
		muted = state.MutedNotificationTypes
	}
	return &dto.NotificationPreferenceResponse{MutedTypes: muted}, nil
}

// UpdateNotificationPreferences replaces the caller's muted set wholesale after
// dropping unknown/duplicate keys. Ensures the state row exists first so a
// brand-new user's first save isn't a silent no-op.
func (s *UserService) UpdateNotificationPreferences(userID int, keys []string) (*dto.NotificationPreferenceResponse, *errors.AppError) {
	clean := msgService.SanitizeMutedKeys(keys)
	if err := s.stateRepo.Ensure(userID); err != nil {
		return nil, errors.ErrInternal("保存通知偏好失败")
	}
	if err := s.stateRepo.UpdateMutedTypes(userID, clean); err != nil {
		return nil, errors.ErrInternal("保存通知偏好失败")
	}
	return &dto.NotificationPreferenceResponse{MutedTypes: clean}, nil
}

// ──────────────────────────────────────────
// Floating hover card
// ──────────────────────────────────────────

func (s *UserService) GetFloatingCard(ctx context.Context, userID int) (*dto.FloatingCardResponse, *errors.AppError) {
	u, ok, err := s.userClient.User(ctx, userID)
	if err != nil {
		return nil, errors.ErrInternal("查询用户信息失败")
	}
	if !ok || u.Status != 0 {
		// Banned or missing — kungal hides the card per agreed policy.
		return nil, errors.ErrNotFound("未找到该用户")
	}

	state, _ := s.stateRepo.FindByID(userID)
	moe := 0
	if state != nil {
		moe = state.Moemoepoint
	}

	stats := s.userStatsRepo.FindFloatingStats(userID)
	// The comment count is local topic_comment + all community comment areas'
	// visible_posts (charter step 06a), replacing the old frozen galgame_comment
	// + galgame_website_comment terms. Cached ~7min so the on-hover card is cheap.
	commentCount := stats.TopicCommentCount + s.communityVisiblePosts(ctx, userID)
	return &dto.FloatingCardResponse{
		ID:                   u.ID,
		Name:                 u.Name,
		Avatar:               u.Avatar,
		Moemoepoint:          moe,
		TopicCount:           stats.TopicCount,
		TopicReplyCount:      stats.TopicReplyCount,
		TopicCommentCount:    commentCount,
		GalgameResourceCount: stats.ResourceCount,
	}, nil
}

// ──────────────────────────────────────────
// Mention autocomplete
// ──────────────────────────────────────────

// SearchMentionUsers backs the editor's @mention dropdown — a thin BFF proxy to
// OAuth /users/search, the C6-sanctioned autocomplete source (never cached). An
// empty query short-circuits to no results so a bare "@" doesn't hit OAuth, and
// limit is clamped to a small autocomplete window. Banned/deleted users (Status
// != 0) are dropped: you can't @mention someone who can't be notified or linked.
func (s *UserService) SearchMentionUsers(ctx context.Context, q string, limit int) ([]dto.MentionUser, *errors.AppError) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []dto.MentionUser{}, nil
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	users, err := s.userClient.SearchUsers(ctx, q, limit)
	if err != nil {
		return nil, errors.ErrInternal("搜索用户失败")
	}
	out := make([]dto.MentionUser, 0, len(users))
	for _, u := range users {
		if u.Status != 0 {
			continue
		}
		out = append(out, dto.MentionUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar})
	}
	return out, nil
}
