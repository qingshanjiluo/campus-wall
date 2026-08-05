package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LikeResult is the toggle outcome for the frontend heart: the new "我赞过" state
// plus the refreshed local like count.
type LikeResult struct {
	Liked     bool `json:"liked"`
	LikeCount int  `json:"like_count"`
}

// LocateResult maps an old galgame_comment id to its migrated community post
// (deep-link continuity).
type LocateResult struct {
	PostID   int64 `json:"post_id"`
	ThreadID int64 `json:"thread_id"`
}

// ──────────────────────────────────────────
// CreateComment
// ──────────────────────────────────────────

// CreateComment resolves the galgame's comments thread (get-or-create) and
// appends a post. replyToPostID is optional; the primitive derives the
// root/target pointers from the parent. After a successful reply it maintains
// the local display counter (galgame.comment_count += 1) and fans out @mention
// notifications deep-linked to the new post (?comment=<post_id> — the old query
// shape, post ids). A held reply (TL0 first-post hold) comes back with held=true
// so the frontend can show "审核中".
func (s *CommunityCommentService) CreateComment(ctx context.Context, userID, galgameID int, content string, replyToPostID *int64) (*CommunityPostItem, *errors.AppError) {
	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: strconv.Itoa(galgameID), ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		return nil, mapCommunityError(err)
	}
	req := communityclient.ReplyRequest{AuthorID: int64(userID), Body: content}
	if replyToPostID != nil {
		req.ReplyToPostID = *replyToPostID
	}
	post, err := s.community.Reply(ctx, thread.Thread.ID, req)
	if err != nil {
		return nil, mapCommunityError(err)
	}

	s.afterCreate(userID, galgameID, content, post)

	// Render through the same read pipeline so held state / pointers / author are
	// consistent with the list (viewer = author, so a held post is visible to it).
	items := s.renderPosts(ctx, galgameID, userID, []communityclient.PostView{*post})
	if len(items) == 0 {
		// The author just authenticated, so this is only reachable if identity
		// resolution transiently failed; build a minimal item rather than 500.
		return buildCommunityItem(*post, galgameID, s.userClient.Hydrate(ctx, []int{userID})[userID], 0, false), nil
	}
	return items[0], nil
}

// afterCreate maintains the local bookkeeping after a reply lands: the lazy
// galgame stub (so the counter UPDATE hits a row), comment_count += 1, and the
// @mention notifications. Best-effort: a failure here never fails the reply (the
// content already committed in the primitive; the display counter tolerates
// drift — charter ruling 11).
func (s *CommunityCommentService) afterCreate(userID, galgameID int, content string, post *communityclient.PostView) {
	root := post.ID
	if post.RootPostID != 0 {
		root = post.RootPostID
	}
	preview := truncate(markdown.StripReferenceTokens(content), 233)
	mentionIDs := markdown.ExtractMentionIDs(content)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.GalgameLocal{ID: galgameID}).Error; e != nil {
			return e
		}
		if e := tx.Model(&model.GalgameLocal{}).Where("id = ?", galgameID).
			Update("comment_count", gorm.Expr("comment_count + 1")).Error; e != nil {
			return e
		}
		for _, mid := range mentionIDs {
			s.helpers.CreateGalgameCommentMention(tx, userID, mid, preview, galgameID, int(post.ID), int(root))
		}
		return nil
	})
	if err != nil {
		slog.Warn("community comment post-create bookkeeping failed (best-effort)", "galgame_id", galgameID, "post_id", post.ID, "error", err)
	}
	// Activity-feed parity (charter ruling 22): the galgame_comment feed trigger
	// carries the galgame id in the gid slot and links the game page.
	feedParityUpsert(s.db, feedTypeGalgameComment, post.ID, userID, galgameID,
		content, "/galgame/"+strconv.Itoa(galgameID), false, post.CreatedAt)
}

// ──────────────────────────────────────────
// UpdateComment
// ──────────────────────────────────────────

// UpdateComment edits a post's body (author-only; a moderator rides the
// mod-actor variant to edit others). galgameID (from the ?gid hint) enables the
// edit-introduced @mention re-fan + populates the returned item; when absent the
// core edit still succeeds and only the local enrichment is skipped. The caller's
// roles are passed through (not a precomputed bool) so the mod-edit authority is
// resolved against THIS post's surface (see resolveModEdit).
func (s *CommunityCommentService) UpdateComment(ctx context.Context, userID int, roles []string, postID int64, galgameID *int, content string) (*CommunityPostItem, *errors.AppError) {
	canModerate := s.resolveModEdit(ctx, userID, roles, postID)

	post, err := s.community.EditPost(ctx, postID, communityclient.EditPostRequest{
		AuthorID: int64(userID), Body: content, AsModerator: canModerate,
	})
	if err != nil {
		return nil, mapCommunityError(err)
	}

	gid := 0
	if galgameID != nil {
		gid = *galgameID
		s.refanMentions(userID, gid, content, post)
	}

	author := s.userClient.Hydrate(ctx, []int{int(post.AuthorID)})[int(post.AuthorID)]
	lc := s.posts.CountLikes([]int64{post.ID})[post.ID]
	liked := s.posts.LikedSet(userID, []int64{post.ID})[post.ID]
	return buildCommunityItem(*post, gid, author, lc, liked), nil
}

// refanMentions re-sends mention notifications for the mentions an edit
// introduced (dedup per link stops re-spam; mirrors the old UpdateComment).
// Best-effort, non-transactional.
func (s *CommunityCommentService) refanMentions(userID, galgameID int, content string, post *communityclient.PostView) {
	root := post.ID
	if post.RootPostID != 0 {
		root = post.RootPostID
	}
	preview := truncate(markdown.StripReferenceTokens(content), 233)
	for _, mid := range markdown.ExtractMentionIDs(content) {
		s.helpers.CreateGalgameCommentMention(s.db, userID, mid, preview, galgameID, int(post.ID), int(root))
	}
}

// resolveModEdit decides whether this edit rides as a moderator action. This
// edit route is SHARED by all four comment surfaces, each with its own
// comment.*.edit key (which runtime overrides can make diverge), so authority
// must be checked against THIS post's surface — not a union. Nothing else in the
// edit flow carries the anchor (EditPost's returned PostView has none), so it
// costs one dedicated S2S read to fetch it. If the anchor can't be resolved (post
// not visible, read failed, or an unrecognized anchor) it falls back to the
// DEFENSIVE union of all four edit keys, so authority is never silently widened
// or dropped. A non-moderator author still edits via EditPost's author match
// (canModerate stays false), so owner-editing-own-comment is untouched.
func (s *CommunityCommentService) resolveModEdit(ctx context.Context, userID int, roles []string, postID int64) bool {
	if resolved, err := s.community.ResolvePosts(ctx, []int64{postID}); err == nil {
		for _, ap := range resolved.Posts {
			if ap.Post.ID != postID {
				continue
			}
			if p, ok := commentEditPermForAnchor(ap.Thread.AnchorKind, ap.Thread.AnchorID); ok {
				return perm.CanUser(userID, roles, p)
			}
		}
	}
	return perm.CanUser(userID, roles, perm.CommentGalgameEdit) ||
		perm.CanUser(userID, roles, perm.CommentRatingEdit) ||
		perm.CanUser(userID, roles, perm.CommentWebsiteEdit) ||
		perm.CanUser(userID, roles, perm.CommentToolsetEdit) ||
		perm.CanUser(userID, roles, perm.CommentResourceEdit) ||
		perm.CanUser(userID, roles, perm.CommentQuizEdit)
}

// commentEditPermForAnchor maps a post's community anchor to the pure-forum edit
// permission that governs its surface: galgame comments anchor site_game, while
// the resource areas ride site_resource with a prefixed anchor id (rating: /
// website: / toolset: / resource: / quiz:). ok=false for an anchor we don't
// recognize (the caller then applies the defensive union).
func commentEditPermForAnchor(anchorKind int32, anchorID string) (perm.Permission, bool) {
	switch anchorKind {
	case communityclient.AnchorSiteGame:
		return perm.CommentGalgameEdit, true
	case communityclient.AnchorSiteResource:
		switch {
		case strings.HasPrefix(anchorID, "rating:"):
			return perm.CommentRatingEdit, true
		case strings.HasPrefix(anchorID, "website:"):
			return perm.CommentWebsiteEdit, true
		case strings.HasPrefix(anchorID, "toolset:"):
			return perm.CommentToolsetEdit, true
		case strings.HasPrefix(anchorID, "resource:"):
			return perm.CommentResourceEdit, true
		case strings.HasPrefix(anchorID, "quiz:"):
			return perm.CommentQuizEdit, true
		}
	}
	return "", false
}

// ──────────────────────────────────────────
// DeleteComment
// ──────────────────────────────────────────

// DeleteComment tombstones a post (author self-delete, or a moderator via the
// mod-actor variant; replies are preserved). It decrements the local display
// counter when the ?gid hint is present (charter ruling 11 tolerates the drift
// when absent).
func (s *CommunityCommentService) DeleteComment(ctx context.Context, userID int, canModerate bool, postID int64, galgameID *int) *errors.AppError {
	if err := s.community.DeletePost(ctx, postID, int64(userID), canModerate); err != nil {
		return mapCommunityError(err)
	}
	if galgameID != nil {
		s.bumpCommentCount(*galgameID, -1)
	}
	// Activity-feed parity (charter ruling 22), including the legacy-keyed row an
	// imported comment carries (its frozen-table DELETE trigger can never fire).
	feedParityDelete(s.db, feedTypeGalgameComment, postID)
	feedParityDeleteLegacyGalgame(s.db, postID)
	return nil
}

// bumpCommentCount adjusts the local display counter, floored at 0 (a tolerated
// display counter; charter ruling 11). Best-effort.
func (s *CommunityCommentService) bumpCommentCount(galgameID, delta int) {
	if err := s.db.Model(&model.GalgameLocal{}).Where("id = ?", galgameID).
		Update("comment_count", gorm.Expr("GREATEST(comment_count + ?, 0)", delta)).Error; err != nil {
		slog.Warn("community comment counter adjust failed (best-effort)", "galgame_id", galgameID, "delta", delta, "error", err)
	}
}

// ──────────────────────────────────────────
// ToggleLike (the triple write)
// ──────────────────────────────────────────

// ToggleLike toggles the like on a community post. The community reaction is the
// AUTHORITATIVE driver: its added/removed result decides all three writes
// (charter ruling 8) — the local galgame_post_like row (display count + "我赞过"),
// the moemoepoint ±1 (self-like not rewarded, matching today), and the "liked"
// notification. Idempotent: the local row uses ON CONFLICT DO NOTHING / a
// no-op delete, so it converges to the primitive's state even after drift.
func (s *CommunityCommentService) ToggleLike(ctx context.Context, userID int, postID int64) (*LikeResult, *errors.AppError) {
	res, err := s.community.ToggleReaction(ctx, postID, communityclient.ReactionToggleRequest{
		UserID: int64(userID), Kind: communityclient.ReactionLike,
	})
	if err != nil {
		return nil, mapCommunityError(err)
	}

	localPresent, awardDelta, notify := likeEffects(res.Added, res.AuthorID, userID)

	if localPresent {
		if e := s.posts.EnsureLike(postID, userID); e != nil {
			slog.Warn("galgame_post_like insert failed", "post_id", postID, "user_id", userID, "error", e)
		}
	} else {
		if e := s.posts.RemoveLike(postID, userID); e != nil {
			slog.Warn("galgame_post_like delete failed", "post_id", postID, "user_id", userID, "error", e)
		}
	}

	if awardDelta != 0 {
		ref := moemoepoint.Ref("galgame_post", int(postID))
		moemoepoint.Award(int(res.AuthorID), awardDelta, moemoepoint.ReasonLiked, ref, moemoepoint.KeyNonce(moemoepoint.ReasonLiked, ref))
	}
	if notify {
		if gid := parseAnchorGid(res.AnchorID); gid > 0 {
			// The reaction response carries no post content, and there is no
			// single-post fetch endpoint, so the preview is empty — the link +
			// dedup (per sender/receiver/galgame) are unchanged, so the "liked"
			// notification still fires and deep-links correctly.
			s.helpers.CreateGalgameMessageWithContent(s.db, userID, int(res.AuthorID), "liked", "", gid)
		}
	}

	lc := s.posts.CountLikes([]int64{postID})[postID]
	return &LikeResult{Liked: res.Added, LikeCount: lc}, nil
}

// likeEffects derives the three side effects of a community reaction toggle from
// its authoritative added/removed result: whether the local like row should be
// present, the moemoepoint delta (+1 on add / -1 on remove), and whether the
// "liked" notification fires. A self-like (author == liker) rewards nothing and
// notifies no one (today's semantics). Pure — unit-tested.
func likeEffects(added bool, authorID int64, userID int) (localPresent bool, awardDelta int, notify bool) {
	self := authorID == int64(userID)
	if added {
		if self {
			return true, 0, false
		}
		return true, 1, true
	}
	if self {
		return false, 0, false
	}
	return false, -1, false
}

// parseAnchorGid reads a galgame id out of a site_game anchor_id (decimal text);
// 0 on a malformed/absent value (the like notification is then skipped).
func parseAnchorGid(anchorID string) int {
	id, err := strconv.Atoi(anchorID)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// ──────────────────────────────────────────
// FlagComment / Locate
// ──────────────────────────────────────────

// FlagComment reports a post to the community moderation queue (governance
// change-over; the UI mount is step 04). reason is the 0-4 enum, note ≤500.
func (s *CommunityCommentService) FlagComment(ctx context.Context, userID int, postID int64, reason int, note string) *errors.AppError {
	if reason < communityclient.FlagReasonSpam || reason > communityclient.FlagReasonNsfwMislabel {
		return errors.ErrValidation("非法的举报理由")
	}
	if err := s.community.SubmitFlag(ctx, postID, communityclient.FlagRequest{
		FlaggerID: int64(userID), Reason: int32(reason), Note: note,
	}); err != nil {
		return mapCommunityError(err)
	}
	return nil
}

// Locate resolves an old galgame_comment id to its migrated community post so
// old deep-links / anchors keep working (step 04 consumes it). 404 when the id
// was never migrated.
func (s *CommunityCommentService) Locate(legacyID int) (*LocateResult, *errors.AppError) {
	row, err := s.posts.FindMapByLegacyID(legacyID)
	if err != nil {
		return nil, errors.ErrInternal("查询失败")
	}
	if row == nil {
		return nil, errors.ErrNotFound("未找到对应评论")
	}
	return &LocateResult{PostID: row.PostID, ThreadID: row.ThreadID}, nil
}
