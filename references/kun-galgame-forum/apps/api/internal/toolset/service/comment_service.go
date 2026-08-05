package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/toolset/dto"
	userModel "kun-galgame-api/internal/user/model"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/userclient"
)

// CommentService serves the toolset detail-page comment PREVIEW off the infra
// community primitive (charter step 06a). The full toolset comment area (list /
// create / edit / delete) is handled by the shared resource-comment BFF on the
// `/comments` routes; the frozen galgame_toolset_comment table was retired
// (migration 060), so this service only reads the primitive.
type CommentService struct {
	userClient *userclient.Client
	community  *communityclient.Client
}

func NewCommentService(
	userClient *userclient.Client,
	community *communityclient.Client,
) *CommentService {
	return &CommentService{userClient: userClient, community: community}
}

// GetLatestForDetail returns the first ≤limit toolset comments for the detail
// preview, sourced from the community primitive. It resolves the toolset's
// comments thread (get-or-create), skips held/tombstoned and banned-author
// posts, and maps onto the existing CommentDetailItem shape. BEST-EFFORT: any
// S2S error (or unconfigured client) yields an empty slice — the detail page
// must always render.
func (s *CommentService) GetLatestForDetail(ctx context.Context, toolsetID, limit int) []dto.CommentDetailItem {
	items := []dto.CommentDetailItem{}
	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind:    communityclient.AnchorSiteResource,
		AnchorID:      "toolset:" + strconv.Itoa(toolsetID),
		ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		slog.Warn("toolset detail: community resolve failed (best-effort)", "toolset_id", toolsetID, "error", err)
		return items
	}

	uids := make([]int, 0, len(thread.Posts))
	for _, p := range thread.Posts {
		uids = append(uids, int(p.AuthorID))
	}
	userMap := s.userClient.Hydrate(ctx, uids)

	for _, p := range thread.Posts {
		if len(items) >= limit {
			break
		}
		if p.Status != communityclient.PostVisible {
			continue
		}
		u := userMap[int(p.AuthorID)]
		if !userclient.IsRenderable(u) {
			continue
		}
		items = append(items, toolsetDetailItemFromPost(p, toolsetID, userBriefFromClient(u)))
	}
	return items
}

// toolsetDetailItemFromPost maps a community post onto the toolset detail-preview
// item shape. Content is the raw markdown (the FE renders it), parent_id mirrors
// the community reply pointer, and the timestamps parse from the RFC3339 wire.
func toolsetDetailItemFromPost(p communityclient.PostView, toolsetID int, user userModel.UserBrief) dto.CommentDetailItem {
	var parentID *int
	if p.ReplyToPostID != 0 {
		pid := int(p.ReplyToPostID)
		parentID = &pid
	}
	created := parseCommunityTime(p.CreatedAt)
	updated := created
	if edited := parseCommunityTime(p.EditedAt); !edited.IsZero() {
		updated = edited
	}
	return dto.CommentDetailItem{
		ID:        int(p.ID),
		Content:   p.ContentRaw,
		UserID:    int(p.AuthorID),
		ToolsetID: toolsetID,
		ParentID:  parentID,
		Edited:    parseCommunityTimePtr(p.EditedAt),
		Created:   created,
		Updated:   updated,
		User:      user,
	}
}

// parseCommunityTime parses an RFC3339 community timestamp, returning the zero
// time on any error (the wire format is RFC3339, but a robust fallback keeps a
// preview from breaking on an odd row).
func parseCommunityTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseCommunityTimePtr returns *time.Time (nil when empty/invalid) for the
// nullable edited_at.
func parseCommunityTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	return nil
}
