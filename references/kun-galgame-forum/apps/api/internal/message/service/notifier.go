package service

import (
	"fmt"
	"strings"

	msgModel "kun-galgame-api/internal/message/model"
	msgRepo "kun-galgame-api/internal/message/repository"

	"gorm.io/gorm"
)

// NotifyKind is the discrete value persisted in message.type. It mirrors the
// set the frontend renders in the notification center.
type NotifyKind string

const (
	NotifyUpvoted   NotifyKind = "upvoted"
	NotifyLiked     NotifyKind = "liked"
	NotifyFavorite  NotifyKind = "favorite"
	NotifyReplied   NotifyKind = "replied"
	NotifyCommented NotifyKind = "commented"
	NotifySolution  NotifyKind = "solution"
	NotifyPinReply  NotifyKind = "pin-reply"
	NotifyMentioned NotifyKind = "mentioned"
	NotifyAdmin     NotifyKind = "admin"
	NotifyExpired   NotifyKind = "expired"
	NotifyRequested NotifyKind = "requested"
	NotifyMerged    NotifyKind = "merged"
	NotifyDeclined  NotifyKind = "declined"
)

// notifyContentLimit matches the varchar(233) column width on message.content.
const notifyContentLimit = 233

// Spec describes a single notification to emit. Link is built from the first
// non-zero target among TopicID, GalgameID, ToolsetID, WebsiteURL (in that
// order) to mirror the legacy Nitro helper.
type Spec struct {
	SenderID   int
	ReceiverID int
	Kind       NotifyKind
	Content    string

	TopicID int
	// ReplyFloor / CommentID deep-link a topic notification to the exact reply
	// (?reply=<floor>) or comment (?comment=<id>) rather than the topic root.
	// Only meaningful alongside TopicID; comment takes precedence (BuildTopicLink).
	ReplyFloor int
	CommentID  int
	GalgameID  int
	ToolsetID  int
	WebsiteURL string
}

// Notifier emits user-to-user notifications. Implementations MUST swallow
// self-notifications and dedup identical (sender, receiver, type, content,
// link) rows so toggling like→unlike→like doesn't spam the receiver.
type Notifier interface {
	Emit(tx *gorm.DB, spec Spec) error
	EmitMany(tx *gorm.DB, specs []Spec) error
}

type notifier struct {
	repo *msgRepo.MessageRepository
}

// NewNotifier wires a Notifier that writes into the project's message table.
func NewNotifier(repo *msgRepo.MessageRepository) Notifier {
	return &notifier{repo: repo}
}

func (n *notifier) Emit(tx *gorm.DB, spec Spec) error {
	if spec.ReceiverID == 0 || spec.SenderID == spec.ReceiverID {
		return nil
	}
	link := buildNotifyLink(spec)
	if link == "" {
		// No target = nothing actionable to surface in the UI.
		return nil
	}
	content := truncateNotifyContent(spec.Content)

	db := tx
	if db == nil {
		db = n.repo.DB()
	}

	var existing int64
	if err := db.Model(&msgModel.Message{}).
		Where(`sender_id = ? AND receiver_id = ? AND type = ? AND content = ? AND link = ?`,
			spec.SenderID, spec.ReceiverID, string(spec.Kind), content, link,
		).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	return db.Create(&msgModel.Message{
		SenderID:   spec.SenderID,
		ReceiverID: spec.ReceiverID,
		Type:       string(spec.Kind),
		Content:    content,
		Link:       link,
	}).Error
}

func (n *notifier) EmitMany(tx *gorm.DB, specs []Spec) error {
	for _, s := range specs {
		if err := n.Emit(tx, s); err != nil {
			return err
		}
	}
	return nil
}

// BuildTopicLink renders a topic notification/feed link, deep-linking to a
// specific post when given one: a comment (?comment=<id>) takes precedence over
// a reply (?reply=<floor>); with neither it's the bare topic root. The query-param
// form matches the reply/comment permalinks the topic page resolves via /locate.
func BuildTopicLink(topicID, replyFloor, commentID int) string {
	switch {
	case commentID > 0:
		return fmt.Sprintf("/topic/%d?comment=%d", topicID, commentID)
	case replyFloor > 0:
		return fmt.Sprintf("/topic/%d?reply=%d", topicID, replyFloor)
	default:
		return fmt.Sprintf("/topic/%d", topicID)
	}
}

func buildNotifyLink(spec Spec) string {
	switch {
	case spec.TopicID > 0:
		return BuildTopicLink(spec.TopicID, spec.ReplyFloor, spec.CommentID)
	case spec.GalgameID > 0:
		return fmt.Sprintf("/galgame/%d", spec.GalgameID)
	case spec.ToolsetID > 0:
		return fmt.Sprintf("/toolset/%d", spec.ToolsetID)
	case spec.WebsiteURL != "":
		return "/website/" + spec.WebsiteURL
	}
	return ""
}

func truncateNotifyContent(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > notifyContentLimit {
		return string(r[:notifyContentLimit])
	}
	return s
}
