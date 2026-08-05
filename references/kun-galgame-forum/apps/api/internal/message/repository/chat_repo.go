package repository

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) DB() *gorm.DB {
	return r.db
}

// ──────────────────────────────────────────
// Row types
// ──────────────────────────────────────────

// RoomListRow is a chat room entry for the contacts sidebar.
type RoomListRow struct {
	ID                 int     `gorm:"column:id"`
	Name               string  `gorm:"column:name"`
	Avatar             string  `gorm:"column:avatar"`
	Type               string  `gorm:"column:type"`
	LastMessageContent string  `gorm:"column:last_message_content"`
	LastMessageTime    *string `gorm:"column:last_message_time"`
}

// ParticipantRow is a row from chat_room_participant. Identity (name/avatar)
// is hydrated by the service layer via userclient.
type ParticipantRow struct {
	ChatRoomID int `gorm:"column:chat_room_id"`
	UserID     int `gorm:"column:user_id"`
}

// CountRow holds a per-room count (unread or total).
type CountRow struct {
	ChatRoomID int `gorm:"column:chat_room_id"`
	Count      int `gorm:"column:count"`
}

// RoomRef is a minimal room reference (id + name).
type RoomRef struct {
	ID   int    `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

// ChatMessageRow is a chat_message row. Identity (sender name/avatar) is
// hydrated by the service layer via userclient.
type ChatMessageRow struct {
	ID           int     `gorm:"column:id"`
	ChatroomName string  `gorm:"column:chatroom_name"`
	SenderID     int     `gorm:"column:sender_id"`
	ReceiverID   int     `gorm:"column:receiver_id"`
	Content      string  `gorm:"column:content"`
	IsRecall     bool    `gorm:"column:is_recall"`
	Created      string  `gorm:"column:created"`
	RecallTime   *string `gorm:"column:recall_time"`
	EditTime     *string `gorm:"column:edit_time"`
}

// ──────────────────────────────────────────
// Room / participant queries
// ──────────────────────────────────────────

// FindRoomsForUser returns every chat room the user participates in that has
// at least one message, ordered by last message time DESC.
func (r *ChatRepository) FindRoomsForUser(userID int) ([]RoomListRow, error) {
	var rooms []RoomListRow
	err := r.db.Table("chat_room cr").
		Select(`cr.id, cr.name, cr.avatar, cr.type,
			cr.last_message_content, cr.last_message_time`).
		Joins("JOIN chat_room_participant crp ON crp.chat_room_id = cr.id").
		Where("crp.user_id = ? AND cr.last_message_sender_id != 0 AND cr.last_message_time IS NOT NULL", userID).
		Order("cr.last_message_time DESC").
		Scan(&rooms).Error
	return rooms, err
}

// FindParticipantsByRoomIDs returns all participants for the given room IDs.
// Identity (name/avatar) is hydrated by the service layer via userclient.
func (r *ChatRepository) FindParticipantsByRoomIDs(roomIDs []int) []ParticipantRow {
	var rows []ParticipantRow
	r.db.Table("chat_room_participant p").
		Select("p.chat_room_id, p.user_id").
		Where("p.chat_room_id IN ?", roomIDs).
		Scan(&rows)
	return rows
}

// CountUnreadByRoomIDs returns unread-message counts (per room) for the given user:
// messages in the room NOT sent by the user AND not present in chat_message_read_by.
func (r *ChatRepository) CountUnreadByRoomIDs(roomIDs []int, userID int) []CountRow {
	var rows []CountRow
	r.db.Table("chat_message cm").
		Select("cm.chat_room_id, COUNT(*) AS count").
		Where("cm.chat_room_id IN ? AND cm.sender_id != ?", roomIDs, userID).
		Where("cm.id NOT IN (SELECT chat_message_id FROM chat_message_read_by WHERE user_id = ?)", userID).
		Group("cm.chat_room_id").
		Scan(&rows)
	return rows
}

// CountTotalByRoomIDs returns total-message counts per room.
func (r *ChatRepository) CountTotalByRoomIDs(roomIDs []int) []CountRow {
	var rows []CountRow
	r.db.Table("chat_message").
		Select("chat_room_id, COUNT(*) AS count").
		Where("chat_room_id IN ?", roomIDs).
		Group("chat_room_id").
		Scan(&rows)
	return rows
}

// FindPrivateRoomBetween looks up the existing private chat room between two
// users by checking the participant table (NOT by room name — names may be
// stale after OAuth migration changed user IDs). Returns the zero value if
// no room exists.
func (r *ChatRepository) FindPrivateRoomBetween(uid1, uid2 int) RoomRef {
	var room RoomRef
	r.db.Raw(`
		SELECT cr.id, cr.name FROM chat_room cr
		WHERE cr.type = 'private'
		AND cr.id IN (
			SELECT chat_room_id FROM chat_room_participant WHERE user_id = ?
		)
		AND cr.id IN (
			SELECT chat_room_id FROM chat_room_participant WHERE user_id = ?
		)
		LIMIT 1`, uid1, uid2).Scan(&room)
	return room
}

// CreatePrivateRoom inserts a new private chat room with both users as
// participants. Returns the new room (id + name); id will be 0 if creation
// failed.
func (r *ChatRepository) CreatePrivateRoom(roomName string, uid1, uid2 int) (RoomRef, error) {
	var room RoomRef
	// Own the clock in Go (like the rest of the app) instead of Postgres NOW():
	// the columns are timestamptz, so this stores a correct absolute instant, and
	// it keeps every chat write on one clock — the NOW()/time.Now() mix is what
	// made these columns zone-inconsistent before migration 023.
	now := time.Now()
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`INSERT INTO chat_room (name, type, created, updated) VALUES (?, 'private', ?, ?)`,
			roomName, now, now,
		).Error; err != nil {
			return err
		}
		if err := tx.Raw(`SELECT id, name FROM chat_room WHERE name = ?`, roomName).Scan(&room).Error; err != nil {
			return err
		}
		if room.ID > 0 {
			if err := tx.Exec(
				`INSERT INTO chat_room_participant (chat_room_id, user_id, created, updated) VALUES (?, ?, ?, ?), (?, ?, ?, ?)`,
				room.ID, uid1, now, now, room.ID, uid2, now, now,
			).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return room, err
}

// ──────────────────────────────────────────
// Chat message queries
// ──────────────────────────────────────────

// FindMessagesByRoom returns chat messages for a room, ordered by id DESC
// (newest first), joined with sender user info. Matches by chat_room_id
// OR legacy chatroom_name (for old data predating the migration).
func (r *ChatRepository) FindMessagesByRoom(roomID int, roomName string, page, limit int) []ChatMessageRow {
	var rows []ChatMessageRow
	offset := (page - 1) * limit
	r.db.Table("chat_message cm").
		Select(`cm.id, cm.chatroom_name, cm.sender_id,
			cm.receiver_id, cm.content, cm.is_recall,
			cm.created, cm.recall_time, cm.edit_time`).
		Where("cm.chat_room_id = ? OR cm.chatroom_name = ?", roomID, roomName).
		Order("cm.id DESC").
		Offset(offset).Limit(limit).
		Scan(&rows)
	return rows
}

// MessageHeader is the slim projection used to validate a recall request:
// who sent the message, in what room, and whether it's already recalled.
// Sender name is hydrated by the service layer via userclient.
type MessageHeader struct {
	ID           int
	ChatRoomID   int    `gorm:"column:chat_room_id"`
	ChatroomName string `gorm:"column:chatroom_name"`
	SenderID     int    `gorm:"column:sender_id"`
	IsRecall     bool   `gorm:"column:is_recall"`
}

// FindMessageHeader loads the header projection for a chat message.
// Returns false on miss; caller maps to ErrNotFound.
func (r *ChatRepository) FindMessageHeader(id int) (MessageHeader, bool) {
	var h MessageHeader
	err := r.db.Table("chat_message m").
		Select(`m.id, m.chat_room_id, m.chatroom_name, m.sender_id, m.is_recall`).
		Where("m.id = ?", id).
		Scan(&h).Error
	if err != nil || h.ID == 0 {
		return MessageHeader{}, false
	}
	return h, true
}

// MarkMessageRecalled flips is_recall + sets recall_time on a chat message
// row. Caller is responsible for ownership / time-window checks.
func (r *ChatRepository) MarkMessageRecalled(id int, now time.Time) error {
	return r.db.Exec(
		`UPDATE chat_message SET is_recall = TRUE, recall_time = ?, updated = ? WHERE id = ?`,
		now, now, id,
	).Error
}

// IsLatestMessageInRoom reports whether the given message is the most
// recent message in its room, used to decide whether to refresh
// chat_room.last_message_content with the recall-preview text.
func (r *ChatRepository) IsLatestMessageInRoom(roomID, msgID int) bool {
	var latest int
	r.db.Table("chat_message").
		Select("MAX(id)").
		Where("chat_room_id = ?", roomID).
		Scan(&latest)
	return latest == msgID
}

// MarkMessagesRead inserts (chat_message_id, user_id) rows into
// chat_message_read_by, ignoring duplicates. A no-op if msgIDs is empty.
// MarkMessagesRead upserts read-receipts for the given messages in ONE
// multi-row INSERT (was a per-message round-trip in a loop). Errors are
// returned so the caller can log; read-receipts are non-critical so the
// caller may choose to continue.
func (r *ChatRepository) MarkMessagesRead(msgIDs []int, userID int) error {
	if len(msgIDs) == 0 {
		return nil
	}
	now := time.Now()
	placeholders := make([]string, 0, len(msgIDs))
	args := make([]any, 0, len(msgIDs)*4)
	for _, mid := range msgIDs {
		placeholders = append(placeholders, "(?, ?, ?, ?)")
		args = append(args, mid, userID, now, now)
	}
	sql := `INSERT INTO chat_message_read_by (chat_message_id, user_id, created, updated) VALUES ` +
		strings.Join(placeholders, ", ") + ` ON CONFLICT DO NOTHING`
	return r.db.Exec(sql, args...).Error
}

// InsertChatMessage / UpdateRoomLastMessage take an explicit executor (the base
// db or a tx) and return their error, so the send path can run both in one
// transaction and surface failures instead of reporting a false "发送成功".
func (r *ChatRepository) InsertChatMessage(db *gorm.DB, roomID int, roomName string, senderID, receiverID int, content string, now time.Time) error {
	return db.Exec(
		`INSERT INTO chat_message (chat_room_id, chatroom_name, sender_id, receiver_id, content, created, updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		roomID, roomName, senderID, receiverID, content, now, now,
	).Error
}

// UpdateRoomLastMessage refreshes chat_room.last_message_* fields.
func (r *ChatRepository) UpdateRoomLastMessage(db *gorm.DB, roomID int, content string, senderID int, senderName string, now time.Time) error {
	return db.Exec(
		`UPDATE chat_room SET last_message_content = ?, last_message_time = ?,
		last_message_sender_id = ?, last_message_sender_name = ?, updated = ? WHERE id = ?`,
		content, now, senderID, senderName, now, roomID,
	).Error
}
