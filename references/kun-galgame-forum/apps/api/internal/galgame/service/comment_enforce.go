package service

import (
	"context"
	"log/slog"

	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/pkg/communityclient"
)

// legacyGalgameCommentMap resolves a legacy galgame_comment id to its migrated
// community (thread, post). CommunityPostRepository satisfies it; the interface
// keeps the enforcer unit-testable without a DB.
type legacyGalgameCommentMap interface {
	FindMapByLegacyID(legacyID int) (*model.GalgameCommentCommunityMap, error)
}

// GalgameCommentEnforcer applies Trust & Safety enforcement for the legacy
// `galgame_comment` subject_kind. The legacy galgame_comment table is retired
// (charter step 06a); its rows were migrated to community posts via the
// galgame_comment_community_map, so enforcement now flows to the primitive:
// resolve the legacy id through the map, then tombstone / look up the author on
// the migrated post. The primitive has no S2S hide, so BOTH `hide` and `remove`
// dispositions map to the tombstone (DeletePost as_moderator) — an acceptable
// strongest-local-action, since reports for comments now run on the primitive's
// own flag system (charter ruling 13) and enforcement callbacks for this legacy
// subject_kind essentially never arrive.
type GalgameCommentEnforcer struct {
	community *communityclient.Client
	posts     legacyGalgameCommentMap
}

func NewGalgameCommentEnforcer(community *communityclient.Client, posts legacyGalgameCommentMap) *GalgameCommentEnforcer {
	return &GalgameCommentEnforcer{community: community, posts: posts}
}

// Tombstone resolves the legacy comment id to its community post and tombstones
// it as a moderator (post_number preserved, replies kept). A never-migrated id
// (no map row) is a logged no-op success. Idempotent: a second tombstone is safe.
func (e *GalgameCommentEnforcer) Tombstone(ctx context.Context, legacyID int) error {
	m, err := e.posts.FindMapByLegacyID(legacyID)
	if err != nil {
		return err
	}
	if m == nil {
		slog.Warn("trust galgame_comment enforcement: no community map row; no-op", "legacy_id", legacyID)
		return nil
	}
	// as_moderator skips the author match, so the author id is advisory; resolve
	// it best-effort for the DELETE audit query param (0 when the post is gone).
	var authorID int64
	if res, rerr := e.community.ResolvePosts(ctx, []int64{m.PostID}); rerr == nil && len(res.Posts) > 0 {
		authorID = res.Posts[0].Post.AuthorID
	}
	return e.community.DeletePost(ctx, m.PostID, authorID, true)
}

// AuthorID resolves the legacy comment id to its migrated post's author (used by
// the warn_user disposition), or 0 when the id was never migrated / the post is
// no longer visible.
func (e *GalgameCommentEnforcer) AuthorID(ctx context.Context, legacyID int) (int, error) {
	m, err := e.posts.FindMapByLegacyID(legacyID)
	if err != nil || m == nil {
		return 0, nil
	}
	res, err := e.community.ResolvePosts(ctx, []int64{m.PostID})
	if err != nil || len(res.Posts) == 0 {
		return 0, nil
	}
	return int(res.Posts[0].Post.AuthorID), nil
}
