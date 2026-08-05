package service

import (
	"context"
	"fmt"
	"log/slog"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/infrastructure/markdown"
	msgModel "kun-galgame-api/internal/message/model"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/errors"

	"gorm.io/gorm"
)

// createCtx carries the per-area data resolved once at create time (so the
// notification / counter / feed side effects don't re-query it).
type createCtx struct {
	galgameID int // rating: the rated galgame (notification deep-link target)
	// ownerID is the resource's owner — the top-level "commented" receiver for the
	// three owner-addressed areas: the toolset owner, the galgame-resource
	// uploader, or the quiz author.
	ownerID int
}

// ──────────────────────────────────────────
// CreateComment
// ──────────────────────────────────────────

// CreateComment resolves the resource's comments thread (get-or-create) and
// appends a post. For the FLAT rating area the caller passes targetUserID (the
// explicit "A → B" recipient, required — charter ruling 19); for the website /
// toolset trees the caller passes replyToPostID and the primitive derives the
// target from the parent. After the post lands it maintains each area's original
// side effects (notification parity, the website display counter, the activity
// feed row) best-effort — a failure there never fails the reply (content is
// already committed in the primitive).
func (s *ResourceCommentService) CreateComment(ctx context.Context, src CommentSource, resourceID, userID int, content string, replyToPostID, targetUserID *int64) (*CommunityPostItem, *errors.AppError) {
	cc, appErr := s.resolveCreateCtx(src, resourceID)
	if appErr != nil {
		return nil, appErr
	}

	// A spoiler-gated quiz refuses writes as well as reads: someone who cannot see
	// the discussion would be posting blind, and letting them post is also a way to
	// probe the thread. Checked AFTER resolveCreateCtx so a missing resource still
	// reports 404 rather than 403.
	if s.commentAreaLocked(ctx, src, resourceID, userID) {
		return nil, errors.ErrForbidden("作答后才能参与这道题目的讨论")
	}

	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteResource, AnchorID: src.anchorID(resourceID), ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		return nil, mapCommunityError(err)
	}

	req := communityclient.ReplyRequest{AuthorID: int64(userID), Body: content}
	if replyToPostID != nil {
		req.ReplyToPostID = *replyToPostID
	}
	if targetUserID != nil {
		req.TargetUserID = *targetUserID
	}
	post, err := s.community.Reply(ctx, thread.Thread.ID, req)
	if err != nil {
		return nil, mapCommunityError(err)
	}

	s.afterCreate(src, resourceID, cc, userID, content, post)

	// Render through the read pipeline (viewer = author, so a held post is
	// visible to it) for shape/pointer/author consistency with the list.
	items := s.renderPosts(ctx, userID, []communityclient.PostView{*post})
	if len(items) == 0 {
		return buildCommunityItem(*post, 0, s.userClient.Hydrate(ctx, []int{userID})[userID], 0, false), nil
	}
	return items[0], nil
}

// resolveCreateCtx does the per-area existence check + resolves the data the side
// effects need. rating: the rating must exist (its galgame id feeds the
// notification link, parity with the old ErrNotFound); toolset / resource / quiz:
// the resource must exist (its owner/author is the top-level "commented"
// receiver); website: no existence check (parity — the old website create never
// verified it).
func (s *ResourceCommentService) resolveCreateCtx(src CommentSource, resourceID int) (createCtx, *errors.AppError) {
	switch src.key {
	case sourceRating.key:
		gid := s.ratingGalgameID(resourceID)
		if gid == 0 {
			return createCtx{}, errors.ErrNotFound("未找到这个评分")
		}
		return createCtx{galgameID: gid}, nil
	case sourceToolset.key:
		owner := s.toolsetOwner(resourceID)
		if owner == 0 {
			return createCtx{}, errors.ErrNotFound("未找到该工具")
		}
		return createCtx{ownerID: owner}, nil
	case sourceResource.key:
		owner := s.galgameResourceOwner(resourceID)
		if owner == 0 {
			return createCtx{}, errors.ErrNotFound("未找到该资源")
		}
		return createCtx{ownerID: owner}, nil
	case sourceQuiz.key:
		author := s.quizAuthor(resourceID)
		if author == 0 {
			return createCtx{}, errors.ErrNotFound("未找到该题目")
		}
		return createCtx{ownerID: author}, nil
	default: // website
		return createCtx{}, nil
	}
}

// notifyPlan is the who + what of a resource comment notification.
type notifyPlan struct {
	receiver int
	msgType  string
}

// resourceNotifyPlan is the PURE per-area notification decision (unit-tested for
// parity, charter ruling 20): rating → the explicit "A → B" target ("commented");
// website → the parent author ONLY on a reply ("commented"), a top-level comment
// notifies nobody; toolset / resource / quiz → the parent author on a reply
// ("replied") else the owner/author ("commented"). ok=false means notify nobody —
// a suppressed self-notification, a missing receiver, or a top-level website
// comment. The derived reply target is the primitive-completed post.TargetUserID.
//
// ownerID is the owner-addressed receiver (toolset owner / resource uploader /
// quiz author); it is ignored by the rating and website branches.
func resourceNotifyPlan(src CommentSource, senderID int, post *communityclient.PostView, ownerID int) (notifyPlan, bool) {
	var p notifyPlan
	switch src.key {
	case sourceRating.key:
		p = notifyPlan{receiver: int(post.TargetUserID), msgType: "commented"}
	case sourceWebsite.key:
		if post.ReplyToPostID == 0 || post.TargetUserID == 0 {
			return notifyPlan{}, false // a top-level website comment notifies nobody
		}
		p = notifyPlan{receiver: int(post.TargetUserID), msgType: "commented"}
	case sourceToolset.key, sourceResource.key, sourceQuiz.key:
		if post.ReplyToPostID != 0 && post.TargetUserID != 0 {
			p = notifyPlan{receiver: int(post.TargetUserID), msgType: "replied"}
		} else {
			p = notifyPlan{receiver: ownerID, msgType: "commented"}
		}
	}
	if p.receiver <= 0 || p.receiver == senderID {
		return notifyPlan{}, false // self / missing receiver — suppressed (parity)
	}
	return p, true
}

// afterCreate replays each area's original create side effects onto the community
// post. Best-effort throughout (charter ruling 11 — the display counter tolerates
// drift; a notification / feed failure never fails the reply).
func (s *ResourceCommentService) afterCreate(src CommentSource, resourceID int, cc createCtx, userID int, content string, post *communityclient.PostView) {
	plan, notify := resourceNotifyPlan(src, userID, post, cc.ownerID)
	switch src.key {
	case sourceRating.key:
		// Reused galgame helper: dedup on (sender,receiver,type,link=/galgame/<gid>).
		if notify {
			s.helpers.CreateGalgameMessageWithContent(s.db, userID, plan.receiver, plan.msgType, truncate(content, constants.TextPreviewLength), cc.galgameID)
		}
		s.feedUpsert(src.feedType, post.ID, userID, content, fmt.Sprintf("/galgame-rating/%d", resourceID), false, post.CreatedAt)

	case sourceWebsite.key:
		s.bumpWebsiteCommentCount(resourceID, 1) // charter ruling 21 — website counter is maintained
		slug, nsfw := s.websiteMeta(resourceID)
		if notify {
			s.notifyWebsiteReply(userID, plan.receiver, content, slug)
		}
		s.feedUpsert(src.feedType, post.ID, userID, content, "/website/"+slug, nsfw, post.CreatedAt)

	case sourceToolset.key:
		s.bumpToolsetCommentCount(resourceID, 1) // charter ruling 21 — toolset counter maintained (migration 059)
		if notify {
			s.notifyToolset(userID, plan.receiver, plan.msgType, content, resourceID)
		}
		s.feedUpsert(src.feedType, post.ID, userID, content, fmt.Sprintf("/toolset/%d", resourceID), false, post.CreatedAt)

	case sourceResource.key:
		s.bumpCountColumn("galgame_resource", resourceID, 1)
		link := fmt.Sprintf("/galgame-resource/%d", resourceID)
		if notify {
			s.notifyDeduped(userID, plan.receiver, plan.msgType, content, link)
		}
		s.feedUpsert(src.feedType, post.ID, userID, content, link, false, post.CreatedAt)

	case sourceQuiz.key:
		s.bumpCountColumn("galgame_quiz", resourceID, 1)
		link := fmt.Sprintf("/galgame-quiz/%d", resourceID)
		if notify {
			s.notifyDeduped(userID, plan.receiver, plan.msgType, content, link)
		}
		s.feedUpsert(src.feedType, post.ID, userID, content, link, false, post.CreatedAt)
	}
}

// ──────────────────────────────────────────
// DeleteComment (region-aware)
// ──────────────────────────────────────────

// DeleteComment tombstones a post from a resource area. The resource id is pinned
// by the route path, so authority is decided SERVER-SIDE (never trusted from the
// client): author self-delete (the primitive checks author_id), a moderator, or —
// for rating (the rated galgame's owner) / toolset (the toolset owner) — the
// resource owner. Owner elevation additionally verifies the post actually belongs
// to THIS resource's thread before bypassing the author check, so a resource
// owner cannot delete an unrelated post by naming their own resource. On success
// it applies the website counter −1 (single-post tombstone semantics, charter
// ruling 11 — no subtree cascade) and removes the activity-feed row (parity with
// the old DELETE trigger).
func (s *ResourceCommentService) DeleteComment(ctx context.Context, src CommentSource, resourceID, userID int, canModerate bool, postID int64) *errors.AppError {
	elevated := canModerate
	if !elevated {
		if owner := s.resourceOwner(src, resourceID); owner != 0 && owner == userID {
			ok, verr := s.postInResourceThread(ctx, src, resourceID, postID)
			if verr != nil {
				return mapCommunityError(verr)
			}
			if !ok {
				return errors.ErrForbidden("您没有权限删除此评论")
			}
			elevated = true
		}
	}

	if err := s.community.DeletePost(ctx, postID, int64(userID), elevated); err != nil {
		return mapCommunityError(err)
	}

	switch src.key {
	case sourceWebsite.key:
		s.bumpWebsiteCommentCount(resourceID, -1)
	case sourceToolset.key:
		s.bumpToolsetCommentCount(resourceID, -1) // charter ruling 21 (migration 059)
	case sourceResource.key:
		s.bumpCountColumn("galgame_resource", resourceID, -1) // migration 065
	case sourceQuiz.key:
		s.bumpCountColumn("galgame_quiz", resourceID, -1) // migration 065
	}
	s.feedDelete(src, postID)
	return nil
}

// resourceOwner returns the user who may delete any comment in this area beyond
// its author: rating → the rated galgame's owner; toolset → the toolset owner;
// resource → the resource's uploader; quiz → the quiz's author; website → none
// (charter ruling 20). 0 = no owner branch / resource missing.
func (s *ResourceCommentService) resourceOwner(src CommentSource, resourceID int) int {
	switch src.key {
	case sourceRating.key:
		gid := s.ratingGalgameID(resourceID)
		if gid == 0 {
			return 0
		}
		return s.galgameOwner(gid)
	case sourceToolset.key:
		return s.toolsetOwner(resourceID)
	case sourceResource.key:
		return s.galgameResourceOwner(resourceID)
	case sourceQuiz.key:
		return s.quizAuthor(resourceID)
	default: // website — no owner branch
		return 0
	}
}

// postInResourceThread reports whether postID belongs to the resource's comments
// thread. Used only on the rare owner-elevated delete path (anti-escalation). The
// resource threads are small, so a bounded keyset scan is cheap.
func (s *ResourceCommentService) postInResourceThread(ctx context.Context, src CommentSource, resourceID int, postID int64) (bool, error) {
	thread, err := s.community.ResolveComments(ctx, communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteResource, AnchorID: src.anchorID(resourceID), ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		return false, err
	}
	if containsPost(thread.Posts, postID) {
		return true, nil
	}
	cursor := thread.NextCursor
	for cursor != "" {
		page, perr := s.community.ListPosts(ctx, thread.Thread.ID, cursor, "50")
		if perr != nil {
			return false, perr
		}
		if containsPost(page.Posts, postID) {
			return true, nil
		}
		cursor = page.NextCursor
	}
	return false, nil
}

func containsPost(posts []communityclient.PostView, postID int64) bool {
	for _, p := range posts {
		if p.ID == postID {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────
// Per-area lookups (raw, to avoid cross-domain repo coupling)
// ──────────────────────────────────────────

func (s *ResourceCommentService) ratingGalgameID(ratingID int) int {
	var gid int
	s.db.Table("galgame_rating").Select("galgame_id").Where("id = ?", ratingID).Scan(&gid)
	return gid
}

func (s *ResourceCommentService) galgameOwner(galgameID int) int {
	var uid int
	s.db.Table("galgame").Select("user_id").Where("id = ?", galgameID).Scan(&uid)
	return uid
}

func (s *ResourceCommentService) toolsetOwner(toolsetID int) int {
	var uid int
	s.db.Table("galgame_toolset").Select("user_id").Where("id = ?", toolsetID).Scan(&uid)
	return uid
}

// galgameResourceOwner returns the resource's uploader (0 when it is gone).
func (s *ResourceCommentService) galgameResourceOwner(resourceID int) int {
	var uid int
	s.db.Table("galgame_resource").Select("user_id").Where("id = ?", resourceID).Scan(&uid)
	return uid
}

// quizAuthor returns the quiz's author (0 when it is gone).
func (s *ResourceCommentService) quizAuthor(quizID int) int {
	var uid int
	s.db.Table("galgame_quiz").Select("user_id").Where("id = ?", quizID).Scan(&uid)
	return uid
}

// websiteMeta resolves the website's url slug + nsfw flag, mirroring the feed
// trigger's projection EXACTLY: a present row is nsfw when age_limit <> 'all'; a
// missing row yields an empty slug (link "/website/") + not-nsfw (the trigger's
// COALESCE(v_nsfw, false)). Existence is read from RowsAffected so an empty
// age_limit on a present row is still treated as nsfw, like `” <> 'all'`.
func (s *ResourceCommentService) websiteMeta(websiteID int) (slug string, nsfw bool) {
	var row struct {
		URL      string `gorm:"column:url"`
		AgeLimit string `gorm:"column:age_limit"`
	}
	res := s.db.Table("galgame_website").Select("url, age_limit").Where("id = ?", websiteID).Limit(1).Find(&row)
	if res.Error != nil || res.RowsAffected == 0 {
		return "", false
	}
	return row.URL, row.AgeLimit != "all"
}

// bumpWebsiteCommentCount adjusts galgame_website.comment_count, floored at 0 (a
// tolerated display counter). Best-effort.
func (s *ResourceCommentService) bumpWebsiteCommentCount(websiteID, delta int) {
	if err := s.db.Table("galgame_website").Where("id = ?", websiteID).
		Update("comment_count", gorm.Expr("GREATEST(comment_count + ?, 0)", delta)).Error; err != nil {
		slog.Warn("website comment counter adjust failed (best-effort)", "website_id", websiteID, "delta", delta, "error", err)
	}
}

// bumpToolsetCommentCount adjusts galgame_toolset.comment_count, floored at 0 (a
// tolerated display counter — migration 059, mirroring the website counter).
// Best-effort.
func (s *ResourceCommentService) bumpToolsetCommentCount(toolsetID, delta int) {
	if err := s.db.Table("galgame_toolset").Where("id = ?", toolsetID).
		Update("comment_count", gorm.Expr("GREATEST(comment_count + ?, 0)", delta)).Error; err != nil {
		slog.Warn("toolset comment counter adjust failed (best-effort)", "toolset_id", toolsetID, "delta", delta, "error", err)
	}
}

// bumpCountColumn adjusts a resource table's comment_count, floored at 0 — the
// generic form used by the two areas introduced with migration 065
// (galgame_resource / galgame_quiz). `table` is a package-internal literal, never
// caller input. Best-effort: a tolerated display counter (charter ruling 11).
func (s *ResourceCommentService) bumpCountColumn(table string, resourceID, delta int) {
	if err := s.db.Table(table).Where("id = ?", resourceID).
		Update("comment_count", gorm.Expr("GREATEST(comment_count + ?, 0)", delta)).Error; err != nil {
		slog.Warn("comment counter adjust failed (best-effort)",
			"table", table, "resource_id", resourceID, "delta", delta, "error", err)
	}
}

// ──────────────────────────────────────────
// Notifications (per-area parity)
// ──────────────────────────────────────────

// notifyWebsiteReply replays the website reply notification: "commented" → parent
// author, link /website/<slug>, content ToPlainText(233), deduped on the full
// (sender,receiver,type,content,link) tuple (parity with the message notifier).
// The receiver is already validated (non-self, >0) by resourceNotifyPlan.
func (s *ResourceCommentService) notifyWebsiteReply(senderID, receiverID int, content, slug string) {
	link := "/website/" + slug
	preview := markdown.ToPlainText(content, constants.TextPreviewLength)
	var count int64
	s.db.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND content = ? AND link = ?",
			senderID, receiverID, "commented", preview, link).
		Count(&count)
	if count > 0 {
		return
	}
	s.db.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type: "commented", Content: preview, Link: link, Status: "unread",
	})
}

// notifyDeduped is the notifier for the two areas introduced WITHOUT a legacy
// table to match (resource / quiz). It follows the website shape — plain-text
// preview capped at constants.TextPreviewLength, deduped on the full
// (sender, receiver, type, content, link) tuple — deliberately rather than the
// toolset shape, whose missing dedup is frozen parity with its old notifier, not
// a behaviour worth copying into a new area. The receiver is already validated
// (non-self, >0) by resourceNotifyPlan.
func (s *ResourceCommentService) notifyDeduped(senderID, receiverID int, msgType, content, link string) {
	preview := markdown.ToPlainText(content, constants.TextPreviewLength)
	var count int64
	s.db.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND content = ? AND link = ?",
			senderID, receiverID, msgType, preview, link).
		Count(&count)
	if count > 0 {
		return
	}
	s.db.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type: msgType, Content: preview, Link: link, Status: "unread",
	})
}

// notifyToolset replays the toolset notification: "replied"/"commented", link
// /toolset/<id>, content ToPlainText(100), NO dedup (parity — the old toolset
// notify created unconditionally). The receiver is already validated (non-self,
// >0) by resourceNotifyPlan.
func (s *ResourceCommentService) notifyToolset(senderID, receiverID int, msgType, content string, toolsetID int) {
	s.db.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type:    msgType,
		Content: markdown.ToPlainText(content, 100),
		Link:    fmt.Sprintf("/toolset/%d", toolsetID),
		Status:  "unread",
	})
}

// ──────────────────────────────────────────
// Activity feed parity (charter ruling 22)
// ──────────────────────────────────────────

// feedUpsert projects a community-backed comment into the materialized feed the
// old table's AFTER-INSERT trigger used to maintain (shared helper in
// feed_parity.go; the resource-comment triggers pass gid=0).
func (s *ResourceCommentService) feedUpsert(feedType string, postID int64, userID int, content, link string, nsfw bool, createdAt string) {
	feedParityUpsert(s.db, feedType, postID, userID, 0, content, link, nsfw, createdAt)
}

// feedDelete removes a comment's feed row on tombstone (parity with the old
// DELETE trigger), including the legacy-keyed row an IMPORTED comment carries.
func (s *ResourceCommentService) feedDelete(src CommentSource, postID int64) {
	feedParityDelete(s.db, src.feedType, postID)
	feedParityDeleteLegacyResource(s.db, src, postID)
}
