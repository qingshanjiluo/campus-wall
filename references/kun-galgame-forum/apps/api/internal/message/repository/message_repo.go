package repository

import (
	"errors"

	"kun-galgame-api/internal/message/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) DB() *gorm.DB {
	return r.db
}

// ──────────────────────────────────────────
// Message CRUD
// ──────────────────────────────────────────

type MessageRow struct {
	ID         int
	SenderID   int
	ReceiverID int
	Link       string
	Content    string
	Status     string
	Type       string
	CreatedAt  string
}

// FindMessages lists a user's notifications. onlyTypes restricts to those
// message.type values (the muted view); excludeTypes drops them (the notice
// view excludes muted categories — migration 053). At most one is set.
func (r *MessageRepository) FindMessages(
	receiverID int,
	excludeTypes, onlyTypes []string,
	sortOrder string,
	page, limit int,
) ([]MessageRow, int64, error) {
	var rows []MessageRow
	var total int64

	query := r.db.Table("message m").
		Select(`m.id, m.sender_id,
			m.receiver_id, m.link, m.content, m.status, m.type, m.created AS created_at`).
		Where("m.receiver_id = ?", receiverID)

	if len(onlyTypes) > 0 {
		query = query.Where("m.type IN ?", onlyTypes)
	}
	if len(excludeTypes) > 0 {
		query = query.Where("m.type NOT IN ?", excludeTypes)
	}

	query.Count(&total)

	err := query.
		Order("m.created " + sortOrder).
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&rows).Error

	return rows, total, err
}

func (r *MessageRepository) DeleteByIDAndReceiver(id, receiverID int) error {
	return r.db.Where("id = ? AND receiver_id = ?", id, receiverID).
		Delete(&model.Message{}).Error
}

func (r *MessageRepository) MarkAllRead(receiverID int) error {
	return r.db.Model(&model.Message{}).
		Where("receiver_id = ? AND status = 'unread'", receiverID).
		Update("status", "read").Error
}

// ──────────────────────────────────────────
// System messages
// ──────────────────────────────────────────

type SystemMessageRow struct {
	ID          int
	ContentEnUS string
	ContentJaJP string
	ContentZhCN string
	ContentZhTW string
	UserID      int
	CreatedAt   string
}

func (r *MessageRepository) FindSystemMessages() ([]SystemMessageRow, error) {
	var rows []SystemMessageRow
	err := r.db.Table("system_message sm").
		Select(`sm.id, sm.content_en_us, sm.content_ja_jp,
			sm.content_zh_cn, sm.content_zh_tw,
			sm.user_id,
			sm.created AS created_at`).
		Order("sm.created DESC").
		Find(&rows).Error
	return rows, err
}

// GetMaxSystemMessageID returns the highest system_message.id currently
// in the table — used by MarkAllSystemRead to advance the user's cursor.
// Returns 0 when there are no broadcasts.
func (r *MessageRepository) GetMaxSystemMessageID() (int64, error) {
	var maxID *int64
	err := r.db.Table("system_message").Select("MAX(id)").Scan(&maxID).Error
	if err != nil || maxID == nil {
		return 0, err
	}
	return *maxID, nil
}

// GetSystemReadCursor returns the user's "read up to" marker, or 0 when
// the user has never marked any broadcast as read.
func (r *MessageRepository) GetSystemReadCursor(userID int) (int64, error) {
	var row model.SystemMessageReadState
	err := r.db.First(&row, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return row.LastReadMessageID, err
}

// UpsertSystemReadCursorForward advances the user's read marker.
//
// GREATEST() ensures the cursor only moves forward — stale "mark all
// read" requests from a tab still showing yesterday's max id can't
// rewind a fresher cursor written by another tab. Mirrors the galgame
// repository pattern (see GalgameMessageRepository.UpsertForward).
func (r *MessageRepository) UpsertSystemReadCursorForward(userID int, lastReadID int64) error {
	return r.db.Exec(`
		INSERT INTO system_message_read_state (user_id, last_read_message_id, updated_at)
		VALUES (?, ?, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET last_read_message_id = GREATEST(
		        system_message_read_state.last_read_message_id,
		        EXCLUDED.last_read_message_id
		    ),
		    updated_at = NOW()
	`, userID, lastReadID).Error
}

// ──────────────────────────────────────────
// Nav summary
// ──────────────────────────────────────────

// GetNavSummary builds the [notice, system] nav badges. localMuted comes from
// the caller's notification preferences (migration 053): muted categories are
// excluded from the notice *unread* count so the badge matches the top-bar red
// dot, while the total count (and the message rows themselves) are left intact —
// muting suppresses the badge, not the record. System broadcasts are non-mutable.
func (r *MessageRepository) GetNavSummary(userID int, localMuted []string) ([]map[string]any, error) {
	// Notice messages — muted categories are excluded across the board (they
	// live in the separate muted view now), so total / unread / preview all
	// skip them and this badge matches the muted-free notice list.
	noticeBase := func() *gorm.DB {
		q := r.db.Model(&model.Message{}).Where("receiver_id = ?", userID)
		if len(localMuted) > 0 {
			q = q.Not("type IN ?", localMuted)
		}
		return q
	}
	var noticeTotal, noticeUnread int64
	noticeBase().Count(&noticeTotal)
	noticeBase().Where("status = 'unread'").Count(&noticeUnread)

	var latestNotice model.Message
	noticeResult := noticeBase().Order("created DESC").First(&latestNotice)

	// System messages — unread is computed against the user's HWM cursor
	// (see migration 012). A missing cursor row means the user has never
	// read anything → treat as cursor=0 so every existing broadcast is
	// unread (matches "new user sees backlog" intent).
	var sysTotal, sysUnread int64
	r.db.Model(&model.SystemMessage{}).Count(&sysTotal)
	cursor, _ := r.GetSystemReadCursor(userID)
	r.db.Model(&model.SystemMessage{}).Where("id > ?", cursor).Count(&sysUnread)

	var latestSys model.SystemMessage
	sysResult := r.db.Order("created DESC").First(&latestSys)

	noticeContent := ""
	if latestNotice.Content != "" {
		if len(latestNotice.Content) > 100 {
			noticeContent = latestNotice.Content[:100]
		} else {
			noticeContent = latestNotice.Content
		}
	}

	var noticeTime any = ""
	if noticeResult.RowsAffected > 0 {
		noticeTime = latestNotice.CreatedAt
	}
	var sysTime any = ""
	if sysResult.RowsAffected > 0 {
		sysTime = latestSys.CreatedAt
	}

	result := []map[string]any{
		{
			"chatroom_name":     "",
			"content":           noticeContent,
			"last_message_time": noticeTime,
			"count":             noticeTotal,
			"unread_count":      noticeUnread,
			"route":             "notice",
			"title":             "zako~",
			"avatar":            "",
		},
		{
			"chatroom_name":     "",
			"content":           "",
			"last_message_time": sysTime,
			"count":             sysTotal,
			"unread_count":      sysUnread,
			"route":             "system",
			"title":             "zako~",
			"avatar":            "",
		},
	}
	return result, nil
}
