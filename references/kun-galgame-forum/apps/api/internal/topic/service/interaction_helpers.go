package service

import (
	"fmt"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/infrastructure/markdown"
	msgModel "kun-galgame-api/internal/message/model"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/moemoepoint"

	"gorm.io/gorm"
)

// InteractionHelpers encapsulates the cross-cutting side effects used by topic
// interactions: moemoepoint adjustment and de-duplicated messaging.
//
// All methods operate on a caller-supplied *gorm.DB (typically a running
// transaction) so the caller controls atomicity.
type InteractionHelpers struct{}

// AdjustMoemoepoint applies a moemoepoint change for `userID`. During the OAuth
// dual-write transition it does BOTH: (1) the local in-tx update on
// kungal_user_state (still authoritative for reads/gating), and (2) a
// non-blocking push to the OAuth single source carrying `reason` (an s2s
// moemoepoint.Reason*) and `ref` (entity, e.g. moemoepoint.Ref("topic", id)).
// The push is async, idempotency-keyed per action, and never blocks/fails the
// caller. No-op when userID<=0 or delta==0. See internal/moemoepoint.
func (InteractionHelpers) AdjustMoemoepoint(_ *gorm.DB, userID, delta int, reason, ref string) {
	// Terminal state: NO local +=. The Awarder calls OAuth (single source) and
	// mirrors the authoritative balance into kungal_user_state; async/best-
	// effort, never blocks. `tx` is unused (kept for call-style uniformity with
	// the messaging helpers); the award does not join the caller's transaction.
	moemoepoint.Award(userID, delta, reason, ref, moemoepoint.KeyNonce(reason, ref))
}

// CreateTopicMessage creates a notification for a topic-level action
// (liked/upvoted/favorite). Dedup is performed on (sender, receiver, type, link).
func (InteractionHelpers) CreateTopicMessage(tx *gorm.DB, senderID, receiverID int, msgType string, topicID int) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := fmt.Sprintf("/topic/%d", topicID)
	createDedupMessage(tx, senderID, receiverID, msgType, "", link)
}

// CreateTopicMessageWithContent is like CreateTopicMessage but stores a content
// preview. replyFloor/commentID deep-link the notification to a specific post
// (pin-reply / a reply-or-comment mention); pass 0/0 for a topic-level target.
func (InteractionHelpers) CreateTopicMessageWithContent(
	tx *gorm.DB,
	senderID, receiverID int,
	msgType, content string,
	topicID, replyFloor, commentID int,
) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := msgService.BuildTopicLink(topicID, replyFloor, commentID)
	createDedupMessage(tx, senderID, receiverID, msgType, content, link)
}

// CreateReplyMessage creates a (non-deduped) reply/commented notification for
// every target/owner separately. Callers should ensure this is only invoked
// once per distinct (sender,receiver).
func (InteractionHelpers) CreateReplyMessage(
	tx *gorm.DB,
	senderID, receiverID int,
	msgType, content string,
	topicID, replyFloor, commentID int,
) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := msgService.BuildTopicLink(topicID, replyFloor, commentID)
	tx.Create(&msgModel.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       msgType,
		Content:    content,
		Link:       link,
		Status:     "unread",
	})
}

// NotifyMentions emits a deduped "mentioned" notification to every user
// @mentioned in `content` (a topic or reply body). replyFloor deep-links the
// notification to the mentioning reply (?reply=<floor>); pass 0 for a topic-body
// mention (topic root). commentID is reserved for comment-body mentions (none are
// wired today). Because the dedup key is (sender, receiver, "mentioned", link),
// each mentioned user is notified once per TARGET per sender — re-editing the same
// post, or @ing the same person twice in it, won't spam; a self-mention is
// skipped. Call inside the write tx.
func (h InteractionHelpers) NotifyMentions(tx *gorm.DB, senderID, topicID, replyFloor, commentID int, content string) {
	// Preview = the reply text with the @/# reference tokens stripped, so the
	// recipient sees what was actually said. A bare mention (no body) yields ""
	// → the frontend falls back to the generic "提到了您".
	preview := truncate(markdown.StripReferenceTokens(content), constants.TextPreviewLength)
	for _, uid := range markdown.ExtractMentionIDs(content) {
		h.CreateTopicMessageWithContent(tx, senderID, uid, "mentioned", preview, topicID, replyFloor, commentID)
	}
}

// createDedupMessage creates a Message only if no row already exists with the
// same (sender, receiver, type, link) triple. Same-user actions are a no-op.
func createDedupMessage(tx *gorm.DB, senderID, receiverID int, msgType, content, link string) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	var count int64
	tx.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND link = ?",
			senderID, receiverID, msgType, link).
		Count(&count)
	if count > 0 {
		return
	}
	tx.Create(&msgModel.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Type:       msgType,
		Content:    content,
		Link:       link,
		Status:     "unread",
	})
}

// recomputeTopicCounts refreshes the denormalized topic.reply_count and
// topic.comment_count from the actual rows. The new BE does not otherwise
// maintain these — the migration backfilled them once, so any reply/comment
// activity since then drifted them (post-migration topics sat at 0; pre-
// migration topics undercount their new replies). See migration 018x backfill.
// Recompute (rather than +1/-1) is intentional: it stays correct through the
// cascade delete that removes a reply plus its nested replies and their
// comments in a single statement. Call it inside the SAME tx as the mutation.
func recomputeTopicCounts(tx *gorm.DB, topicID int) error {
	// status = 0 excludes moderation-hidden rows from the visible counts
	// (T&S `hide`, migration 055).
	return tx.Exec(`
		UPDATE topic SET
			reply_count   = (SELECT COUNT(*) FROM topic_reply  WHERE topic_id = ? AND status = 0),
			comment_count = (SELECT COUNT(*) FROM topic_comment WHERE topic_id = ? AND status = 0)
		WHERE id = ?`, topicID, topicID, topicID).Error
}

// truncate trims `s` to `maxLen` runes (rune-aware, avoids splitting multibyte chars).
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
