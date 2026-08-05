package service

// ResourceCommentService is the BFF for the resource comment areas — rating /
// website / toolset (rerouted off frozen local tables in charter step 07) plus
// galgame-resource / quiz (introduced directly on the primitive, so they have no
// legacy parity to preserve). It mirrors CommunityCommentService (the proven
// galgame comment reroute in this package) and REUSES its pure render helpers
// (buildCommunityItem / visibleTo / collectPostUserIDs / mapCommunityError /
// isCommunityDown / clampReadLimit) verbatim, so the two stay behaviourally
// identical. The primitive owns thread/post content over S2S; this service
// assembles the response shapes kungal already renders plus the per-area side
// effects each old area maintained: notifications (per-area parity), the website
// display counter, and the materialized activity-feed row. It is the
// unconditional rating/website/toolset comment backend: the OLD singular comment
// routes were retired (charter step 06a); the frozen tables still back detail-
// embedded / admin / search reads.
//
// Split across resource_comment_service.go (source strategy + read) and
// resource_comment_write.go (create/delete + notifications + feed parity).

import (
	"context"
	"strconv"

	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

type ResourceCommentService struct {
	community  *communityclient.Client
	posts      *repository.CommunityPostRepository
	userClient *userclient.Client
	db         *gorm.DB
	helpers    InteractionHelpers
}

func NewResourceCommentService(
	community *communityclient.Client,
	posts *repository.CommunityPostRepository,
	userClient *userclient.Client,
	db *gorm.DB,
) *ResourceCommentService {
	return &ResourceCommentService{community: community, posts: posts, userClient: userClient, db: db}
}

// ──────────────────────────────────────────
// Source strategy
// ──────────────────────────────────────────

// CommentSource is the per-area strategy: the db/path discriminator, the anchor
// id prefix (all three ride anchor_kind=2 site_resource, charter ruling 18), and
// the feed_activity type the old table's trigger emitted (charter ruling 22 — the
// create/delete paths keep that projection alive by hand, since community posts
// live in another database and never fire the trigger). The content length cap
// (rating 1314, website/toolset 1007 — parity with the old DTO) is enforced by
// the handler's validate tags, the single enforcement point.
type CommentSource struct {
	key        string // "rating" | "website" | "toolset" | "resource" | "quiz"
	anchorPref string // "rating:" | "website:" | "toolset:" | "resource:" | "quiz:"
	feedType   string // GALGAME_RATING_COMMENT_CREATION | GALGAME_WEBSITE_COMMENT_CREATION | TOOLSET_COMMENT_CREATION | GALGAME_RESOURCE_COMMENT_CREATION | GALGAME_QUIZ_COMMENT_CREATION
}

var (
	sourceRating  = CommentSource{key: "rating", anchorPref: "rating:", feedType: "GALGAME_RATING_COMMENT_CREATION"}
	sourceWebsite = CommentSource{key: "website", anchorPref: "website:", feedType: "GALGAME_WEBSITE_COMMENT_CREATION"}
	sourceToolset = CommentSource{key: "toolset", anchorPref: "toolset:", feedType: "TOOLSET_COMMENT_CREATION"}
	// resource / quiz are NEW areas (not migrated from a frozen table), so they
	// have no legacy feed trigger to mirror — their feed types are introduced with
	// them, and the anchor prefixes follow the same short-domain convention.
	sourceResource = CommentSource{key: "resource", anchorPref: "resource:", feedType: "GALGAME_RESOURCE_COMMENT_CREATION"}
	sourceQuiz     = CommentSource{key: "quiz", anchorPref: "quiz:", feedType: "GALGAME_QUIZ_COMMENT_CREATION"}
)

// SourceRating / SourceWebsite / SourceToolset / SourceResource / SourceQuiz
// expose the strategies to the handler layer (which selects one per route).
func SourceRating() CommentSource   { return sourceRating }
func SourceWebsite() CommentSource  { return sourceWebsite }
func SourceToolset() CommentSource  { return sourceToolset }
func SourceResource() CommentSource { return sourceResource }
func SourceQuiz() CommentSource     { return sourceQuiz }

// anchorID builds the site_resource anchor for a resource id, e.g. "rating:42".
func (src CommentSource) anchorID(resourceID int) string {
	return src.anchorPref + strconv.Itoa(resourceID)
}

// ──────────────────────────────────────────
// GetComments (read)
// ──────────────────────────────────────────

// GetComments returns a flat keyset page of a resource's comments — the same
// shape as the galgame comment read (CommunityCommentPage), so the frontend
// reuses the two-tier grouping. Page 1 comes from the thread resolve
// (get-or-create); later pages from listPosts. A down/unconfigured community
// degrades to an empty page (fail-closed to the face). resourceID is the
// rating/website/toolset/resource/quiz id; cursor is the post_number `after`;
// limit ≤50.
//
// A spoiler-gated quiz (see resource_comment_gate.go) returns an EMPTY page with
// Locked=true for a viewer who has not answered it, short-circuiting before the
// thread resolve — so the answer never reaches the wire and no empty thread is
// created for a viewer who cannot post anyway.
func (s *ResourceCommentService) GetComments(ctx context.Context, src CommentSource, resourceID, viewerID int, cursor string, limit int) (*CommunityCommentPage, *errors.AppError) {
	if s.commentAreaLocked(ctx, src, resourceID, viewerID) {
		return lockedCommentPage(), nil
	}

	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteResource, AnchorID: src.anchorID(resourceID), ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		if isCommunityDown(err) {
			return emptyCommentPage(), nil
		}
		return nil, mapCommunityError(err)
	}

	posts, next := thread.Posts, thread.NextCursor
	if cursor != "" {
		page, perr := s.community.ListPosts(ctx, thread.Thread.ID, cursor, clampReadLimit(limit))
		if perr != nil {
			if isCommunityDown(perr) {
				return emptyCommentPage(), nil
			}
			return nil, mapCommunityError(perr)
		}
		posts, next = page.Posts, page.NextCursor
	}

	items := s.renderPosts(ctx, viewerID, posts)
	return &CommunityCommentPage{
		ThreadID: thread.Thread.ID, Posts: items, NextCursor: next, Total: int(thread.Thread.PostsCount),
	}, nil
}

// renderPosts enriches a community post page into the flat item shape, reusing
// the galgame comment mapper's helpers so the two areas render identically:
// batch OAuth identity, local like_count + is_liked, banned-author skip, held
// self-see, and deleted-placeholder. galgame_id is left 0 on the resource items
// (not a galgame-scoped card); the "A → B" target_user is projected for the flat
// rating area (and any reply that carries a parent author).
func (s *ResourceCommentService) renderPosts(ctx context.Context, viewerID int, posts []communityclient.PostView) []*CommunityPostItem {
	userMap := s.userClient.Hydrate(ctx, collectPostUserIDs(posts))

	postIDs := make([]int64, 0, len(posts))
	for _, p := range posts {
		postIDs = append(postIDs, p.ID)
	}
	likeCounts := s.posts.CountLikes(postIDs)
	likedSet := s.posts.LikedSet(viewerID, postIDs)

	out := make([]*CommunityPostItem, 0, len(posts))
	for _, p := range posts {
		author := userMap[int(p.AuthorID)]
		if !userclient.IsRenderable(author) {
			continue // banned/deleted author — hidden at the mapper layer, like today
		}
		if !visibleTo(p.Status, p.AuthorID, viewerID) {
			continue // a held post is author-only; others never see it
		}
		item := buildCommunityItem(p, 0, author, likeCounts[p.ID], likedSet[p.ID])
		if p.TargetUserID != 0 {
			t := userMap[int(p.TargetUserID)]
			tu := UserObj{ID: t.ID, Name: t.Name, Avatar: t.Avatar}
			item.TargetUser = &tu
		}
		out = append(out, item)
	}
	return out
}
