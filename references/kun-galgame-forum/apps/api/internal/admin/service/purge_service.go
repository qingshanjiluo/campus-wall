package service

import (
	"context"
	"log/slog"

	"kun-galgame-api/internal/admin/dto"
	"kun-galgame-api/internal/admin/repository"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/role"
	"kun-galgame-api/pkg/userclient"
)

type PurgeService struct {
	repo       *repository.PurgeRepository
	userClient *userclient.Client
	community  *communityclient.Client
}

func NewPurgeService(repo *repository.PurgeRepository, userClient *userclient.Client, community *communityclient.Client) *PurgeService {
	return &PurgeService{repo: repo, userClient: userClient, community: community}
}

// GetUserContentStats previews what a purge would remove: the local per-type
// breakdown plus the user's community-primitive visible post count (best-effort;
// 0 when the community face is down — a preview must never fail).
func (s *PurgeService) GetUserContentStats(ctx context.Context, userID int) dto.UserContentStats {
	stats := s.repo.CountUserContent(userID)
	if resp, err := s.community.AuthorStats(ctx, []int64{int64(userID)}); err == nil {
		for _, st := range resp.Stats {
			if st.AuthorID == int64(userID) {
				stats.CommunityPosts = st.VisiblePosts
				break
			}
		}
	}
	return stats
}

// PurgeUserContent hard-deletes all of the user's kungal content +
// interactions and returns the breakdown of what was removed. operatorID is the
// acting admin (for the audit line).
//
// Guard: privileged accounts (anyone with the moderation capability —
// moderators / admins) are NOT purgeable. Their content includes site
// documentation / changelogs that other users read, and the feature exists for
// spam/ad accounts only. We resolve the TARGET user's roles from OAuth (the
// role source of truth). On an OAuth lookup error we refuse (fail-safe — never
// purge during an outage if we can't confirm the target isn't privileged); a
// not-found user is treated as a gone normal account whose leftover content may
// still be purged.
//
// Order: the local transaction runs first (frozen legacy tables included —
// charter ruling 22), then the community compliance purge (tombstone + scrub +
// reaction delete). A community failure surfaces as an error so the admin
// retries — both sides are idempotent, so a retry is safe.
func (s *PurgeService) PurgeUserContent(ctx context.Context, operatorID, userID int) (dto.PurgeResult, *errors.AppError) {
	u, found, err := s.userClient.User(ctx, userID)
	if err != nil {
		return dto.PurgeResult{}, errors.ErrInternal("无法核验用户身份, 已中止清除")
	}
	// This is a guard on the TARGET's status (never purge staff content), not
	// the operator's authorization — the operator's user.purge_content gate is
	// enforced at the router. So it stays role.CanModerate: it asks "is the
	// target privileged?", which is an identity/capability property, not a
	// pkg/perm forum-operation check.
	if found && role.CanModerate(u.Roles) {
		return dto.PurgeResult{}, errors.ErrForbidden("不可清除管理员 / 版主用户的内容")
	}

	stats, dbErr := s.repo.PurgeUserContent(userID)
	if dbErr != nil {
		return dto.PurgeResult{}, errors.ErrInternal("清除用户内容失败")
	}

	// Community compliance purge (charter step 06a). AuthorPurge is idempotent, so
	// a retry after a transient S2S failure is safe.
	purged, cErr := s.community.AuthorPurge(ctx, int64(userID))
	if cErr != nil {
		slog.Error("purge: community AuthorPurge failed — local delete done, community pending; admin should retry",
			"operator_id", operatorID, "target_id", userID, "local_total", stats.Total, "error", cErr)
		return dto.PurgeResult{}, errors.ErrInternal("已清除本地内容, 但社区内容清除失败, 请重试")
	}

	slog.Info("purge: user content purged",
		"operator_id", operatorID, "target_id", userID,
		"local_total", stats.Total,
		"community_posts_purged", purged.PostsPurged,
		"community_reactions_deleted", purged.ReactionsDeleted)

	return dto.PurgeResult{
		UserContentStats:          stats,
		CommunityPostsPurged:      purged.PostsPurged,
		CommunityReactionsDeleted: purged.ReactionsDeleted,
	}, nil
}
