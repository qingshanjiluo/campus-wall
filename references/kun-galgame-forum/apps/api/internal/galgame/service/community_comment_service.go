package service

// CommunityCommentService is the BFF for the galgame comment area once it is
// rerouted onto the infra community primitive (charter step 03). The primitive
// is the source of truth for thread/post content over S2S; this service
// assembles the response shapes kungal already renders on top of it — author
// identity from OAuth (userclient), like_count + "我赞过" from the LOCAL
// galgame_post_like table, content HTML from kungal's OWN markdown pipeline
// (charter ruling 7: kungal reads content_raw, never the primitive's
// content_html). It is the unconditional galgame comment backend: the OLD
// galgame_comment routes were retired and the frozen galgame_comment table was
// dropped (charter step 06a; migration 060) — every reader now reads the
// primitive. Degradation is fail-closed to the face (empty read / 503 write),
// never a local-table fallback.
//
// Split across community_comment_service.go (read + shared) and
// community_comment_write.go (create/edit/delete/like/flag/locate).

import (
	"context"
	stderrors "errors"
	"log/slog"
	"strconv"

	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

type CommunityCommentService struct {
	community  *communityclient.Client
	posts      *repository.CommunityPostRepository
	userClient *userclient.Client
	db         *gorm.DB
	helpers    InteractionHelpers
}

func NewCommunityCommentService(
	community *communityclient.Client,
	posts *repository.CommunityPostRepository,
	userClient *userclient.Client,
	db *gorm.DB,
) *CommunityCommentService {
	return &CommunityCommentService{community: community, posts: posts, userClient: userClient, db: db}
}

// ──────────────────────────────────────────
// Wire shapes
// ──────────────────────────────────────────

// UserObj is the minimal author identity embedded in every comment wire shape
// (galgame community + resource comments). Relocated here from the retired
// legacy galgame comment service (charter step 06a).
type UserObj struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// CommunityPostItem is one flat comment. root/reply pointers are exposed
// (parent_comment_id = the community reply_to_post_id, root_comment_id = the
// community root_post_id); the frontend (step 04) does the two-tier grouping.
// A deleted post's content is blanked (a tombstone is never served) and
// status/deleted/held drive the "[已删除]" placeholder + "审核中" rendering.
type CommunityPostItem struct {
	ID                int64    `json:"id"`
	Content           string   `json:"content"`
	ContentHtml       string   `json:"content_html"`
	GalgameID         int      `json:"galgame_id"`
	User              UserObj  `json:"user"`
	ParentCommentID   *int64   `json:"parent_comment_id"`
	RootCommentID     *int64   `json:"root_comment_id"`
	TargetUser        *UserObj `json:"target_user,omitempty"`
	LikeCount         int      `json:"like_count"`
	IsLiked           bool     `json:"is_liked"`
	Created           string   `json:"created"`
	Edited            *string  `json:"edited"`
	EditedByModerator bool     `json:"edited_by_moderator"`
	Status            int32    `json:"status"`
	Deleted           bool     `json:"deleted"`
	Held              bool     `json:"held"`
}

// CommunityCommentPage is a flat keyset page of a galgame's comments. Total =
// the thread's posts_count (includes held/deleted numbers), matching the
// primitive's counter; next_cursor is a post_number sentinel ("" = last page).
type CommunityCommentPage struct {
	ThreadID   int64                `json:"thread_id"`
	Posts      []*CommunityPostItem `json:"posts"`
	NextCursor string               `json:"next_cursor"`
	Total      int                  `json:"total"`
	// Locked reports that the area is withheld from THIS viewer behind a spoiler
	// gate (currently only an unanswered, concealing quiz — see
	// resource_comment_gate.go). Always false for every other area. When true the
	// page carries no posts and the client renders the gate placeholder.
	Locked bool `json:"locked"`
}

// ──────────────────────────────────────────
// GetComments (read)
// ──────────────────────────────────────────

// GetComments returns a flat keyset page of a galgame's comments. Page 1 comes
// from the thread resolve (get-or-create); later pages from listPosts on the
// resolved thread. A down/unconfigured community degrades to an empty page
// rather than a 5xx (fail-closed to the face). galgameID identifies the anchor;
// cursor is the post_number `after` ("" = first page); limit is clamped ≤50.
func (s *CommunityCommentService) GetComments(ctx context.Context, galgameID, viewerID int, cursor string, limit int) (*CommunityCommentPage, *errors.AppError) {
	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: strconv.Itoa(galgameID), ContentRating: communityclient.RatingAll,
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

	items := s.renderPosts(ctx, galgameID, viewerID, posts)
	return &CommunityCommentPage{
		ThreadID: thread.Thread.ID, Posts: items, NextCursor: next, Total: int(thread.Thread.PostsCount),
	}, nil
}

// renderPosts enriches a community post page into the flat item shape: author
// identity (batch OAuth), like_count + is_liked (batch local), locally-rendered
// content HTML, and the held-self-see / deleted-placeholder filtering. Posts
// authored by a banned/deleted user are dropped (mirrors the old comment
// mapper's IsRenderable skip), and a held post is dropped for everyone but its
// own author.
func (s *CommunityCommentService) renderPosts(ctx context.Context, galgameID, viewerID int, posts []communityclient.PostView) []*CommunityPostItem {
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
		item := buildCommunityItem(p, galgameID, author, likeCounts[p.ID], likedSet[p.ID])
		if p.TargetUserID != 0 {
			t := userMap[int(p.TargetUserID)]
			tu := UserObj{ID: t.ID, Name: t.Name, Avatar: t.Avatar}
			item.TargetUser = &tu
		}
		out = append(out, item)
	}
	return out
}

// ──────────────────────────────────────────
// Pure helpers (unit-tested in community_comment_test.go)
// ──────────────────────────────────────────

// visibleTo filters a held post (status=1) to its author only — a TL0 newcomer
// sees their own "审核中" post, nobody else does. Tombstones (rendered as a
// placeholder) and visible posts pass. viewerID 0 = anonymous.
func visibleTo(status int32, authorID int64, viewerID int) bool {
	if status == communityclient.PostHeld {
		return viewerID != 0 && int64(viewerID) == authorID
	}
	return true
}

// buildCommunityItem maps one community post to the flat item shape. A deleted
// post's content is blanked (never serve a tombstone's body); content HTML is
// rendered from the raw markdown through kungal's own pipeline (ruling 7).
func buildCommunityItem(p communityclient.PostView, galgameID int, author userclient.User, likeCount int, liked bool) *CommunityPostItem {
	deleted := p.Status == communityclient.PostDeleted
	raw := p.ContentRaw
	if deleted {
		raw = ""
	}
	item := &CommunityPostItem{
		ID:                p.ID,
		Content:           raw,
		ContentHtml:       markdown.Render(raw),
		GalgameID:         galgameID,
		User:              UserObj{ID: author.ID, Name: author.Name, Avatar: author.Avatar},
		ParentCommentID:   nzPtr64(p.ReplyToPostID),
		RootCommentID:     nzPtr64(p.RootPostID),
		LikeCount:         likeCount,
		IsLiked:           liked,
		Created:           p.CreatedAt,
		EditedByModerator: p.EditedByModerator,
		Status:            p.Status,
		Deleted:           deleted,
		Held:              p.Status == communityclient.PostHeld,
	}
	if p.EditedAt != "" {
		e := p.EditedAt
		item.Edited = &e
	}
	return item
}

// clampReadLimit clamps a read page size to 1..50 (spec cap) and stringifies it
// for the community query; 0/out-of-range → the 50 default.
func clampReadLimit(limit int) string {
	if limit < 1 || limit > 50 {
		limit = 50
	}
	return strconv.Itoa(limit)
}

// collectPostUserIDs gathers the distinct author + @target ids of a page for one
// batch OAuth resolve.
func collectPostUserIDs(posts []communityclient.PostView) []int {
	set := make(map[int]struct{}, len(posts))
	for _, p := range posts {
		set[int(p.AuthorID)] = struct{}{}
		if p.TargetUserID != 0 {
			set[int(p.TargetUserID)] = struct{}{}
		}
	}
	ids := make([]int, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

// nzPtr64 returns a *int64 for a non-zero community id (0 → nil) so the JSON
// serializes null — matching the frontend's `number | null` reply/root keys.
func nzPtr64(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

func emptyCommentPage() *CommunityCommentPage {
	return &CommunityCommentPage{Posts: []*CommunityPostItem{}}
}

// lockedCommentPage is the withheld page a spoiler-gated area serves: no posts,
// no counts, and Locked=true so the client renders the "answer first" placeholder
// from the server's ruling rather than re-deriving the rule.
func lockedCommentPage() *CommunityCommentPage {
	return &CommunityCommentPage{Posts: []*CommunityPostItem{}, Locked: true}
}

// truncate clamps a string to maxLen runes (CJK-safe). Package-shared preview
// helper (notification/feed bodies); relocated here from the retired legacy
// galgame comment service (charter step 06a).
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// ──────────────────────────────────────────
// Error mapping / degradation
// ──────────────────────────────────────────

// isCommunityDown reports whether an error means the community service is
// unreachable/unconfigured/forbidden — the cases where a READ degrades to an
// empty page rather than a 5xx.
func isCommunityDown(err error) bool {
	if stderrors.Is(err, communityclient.ErrNotConfigured) || stderrors.Is(err, communityclient.ErrForbidden) {
		return true
	}
	// A transport failure (service unreachable) surfaces as a wrapped non-API,
	// non-rate-limited error.
	var apiErr *communityclient.APIError
	return err != nil && !stderrors.As(err, &apiErr) && !stderrors.Is(err, communityclient.ErrRateLimited)
}

// mapCommunityError translates a communityclient error onto the house AppError
// with the right status: the TL0 sandbox cap → 429; an author-only denial (403)
// → 403; not-configured/transport → 503; a 4xx business error → its message at
// 400. There is deliberately no local-table fallback.
func mapCommunityError(err error) *errors.AppError {
	switch {
	case stderrors.Is(err, communityclient.ErrRateLimited):
		return errors.New(errors.CodeBiz, "发表过于频繁，请稍后再试（新人限制）", 429)
	case stderrors.Is(err, communityclient.ErrForbidden):
		return errors.New(errors.CodeBiz, "你没有权限执行此操作", 403)
	case stderrors.Is(err, communityclient.ErrNotConfigured):
		return errors.New(errors.CodeBiz, "评论服务暂不可用", 503)
	}
	var apiErr *communityclient.APIError
	if stderrors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return errors.New(apiErr.Code, apiErr.Msg, 400)
	}
	slog.Error("community upstream error", "error", err)
	return errors.New(errors.CodeBiz, "评论服务暂不可用", 503)
}
