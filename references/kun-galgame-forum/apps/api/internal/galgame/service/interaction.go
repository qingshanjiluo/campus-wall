package service

import (
	"fmt"

	msgModel "kun-galgame-api/internal/message/model"
	"kun-galgame-api/internal/moemoepoint"

	"gorm.io/gorm"
)

// InteractionHelpers encapsulates the two cross-cutting side effects used by
// galgame interactions: moemoepoint adjustment and de-duplicated messaging.
//
// It operates on a caller-supplied *gorm.DB (typically a running transaction)
// so the caller controls atomicity.
type InteractionHelpers struct{}

// AdjustMoemoepoint applies a moemoepoint change for `userID` via the OAuth
// single source (terminal state — NO local +=). The Awarder calls OAuth and
// mirrors the returned authoritative balance into kungal_user_state; it's
// async/best-effort and never blocks the caller. `reason` is an s2s
// moemoepoint.Reason*; `ref` is the entity (e.g. moemoepoint.Ref("galgame",
// id)). Per-action nonce key (user-initiated, non-replayed → no false dedup).
// The `tx` param is unused (kept so the messaging helpers' call style is
// uniform); the award deliberately does NOT join the caller's transaction.
func (InteractionHelpers) AdjustMoemoepoint(_ *gorm.DB, userID, delta int, reason, ref string) {
	moemoepoint.Award(userID, delta, reason, ref, moemoepoint.KeyNonce(reason, ref))
}

// CreateGalgameMessage creates a notification from `senderID` → `receiverID`
// for a galgame-related action, deduplicating against any existing row with
// the same (sender, receiver, type, link) triple. Same-user actions are a no-op.
func (h InteractionHelpers) CreateGalgameMessage(tx *gorm.DB, senderID, receiverID int, msgType string, galgameID int) {
	h.CreateGalgameMessageWithContent(tx, senderID, receiverID, msgType, "", galgameID)
}

// CreateGalgameMessageWithContent is the variant that records a content
// preview (used by resource liked/expired notifications). Dedup applies on
// (sender, receiver, type, link) — content is informational only.
func (InteractionHelpers) CreateGalgameMessageWithContent(
	tx *gorm.DB,
	senderID, receiverID int,
	msgType, content string,
	galgameID int,
) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := fmt.Sprintf("/galgame/%d", galgameID)

	var count int64
	tx.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND link = ?",
			senderID, receiverID, msgType, link).
		Count(&count)
	if count > 0 {
		return
	}

	tx.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type: msgType, Content: content, Link: link, Status: "unread",
	})
}

// CreateQuizAnswerMessage notifies a quiz's author that `senderID` just answered
// their quiz, carrying a preview of the answerer's choice + verdict. Link → the
// quiz; dedups on (sender, receiver, "quiz-answered", link). No-op on self / no
// receiver.
func (InteractionHelpers) CreateQuizAnswerMessage(
	tx *gorm.DB,
	senderID, receiverID int,
	content string,
	quizID int,
) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := fmt.Sprintf("/galgame-quiz/%d", quizID)

	var count int64
	tx.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND link = ?",
			senderID, receiverID, "quiz-answered", link).
		Count(&count)
	if count > 0 {
		return
	}

	tx.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type: "quiz-answered", Content: content, Link: link, Status: "unread",
	})
}

// CreateGalgameCommentMention notifies a user @-mentioned in a galgame comment,
// deep-linked to that comment so clicking the notification lands on it:
// ?comment=<id> is the comment to highlight, &thread=<rootID> is the thread's
// root so the FE can open the thread drawer (which loads the whole thread) and
// scroll to the comment regardless of pagination. rootID == commentID for a
// top-level comment. content is the already-stripped preview. Dedups on
// (sender, receiver, "mentioned", link) — per-comment. No-op on self.
func (InteractionHelpers) CreateGalgameCommentMention(
	tx *gorm.DB,
	senderID, receiverID int,
	content string,
	galgameID, commentID, rootID int,
) {
	if senderID == receiverID || receiverID <= 0 {
		return
	}
	link := fmt.Sprintf("/galgame/%d?comment=%d&thread=%d", galgameID, commentID, rootID)

	var count int64
	tx.Model(&msgModel.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND type = ? AND link = ?",
			senderID, receiverID, "mentioned", link).
		Count(&count)
	if count > 0 {
		return
	}

	tx.Create(&msgModel.Message{
		SenderID: senderID, ReceiverID: receiverID,
		Type: "mentioned", Content: content, Link: link, Status: "unread",
	})
}
