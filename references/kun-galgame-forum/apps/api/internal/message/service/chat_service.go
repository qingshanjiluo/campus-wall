package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/message/dto"
	"kun-galgame-api/internal/message/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

type ChatService struct {
	chatRepo   *repository.ChatRepository
	userClient *userclient.Client
}

func NewChatService(
	chatRepo *repository.ChatRepository,
	userClient *userclient.Client,
) *ChatService {
	return &ChatService{chatRepo: chatRepo, userClient: userClient}
}

// ──────────────────────────────────────────
// GetNavContact — GET /api/message/nav/contact
// ──────────────────────────────────────────

// GetNavContact returns the chat room list for the message sidebar.
// For private rooms, the display title/avatar/route are resolved to the
// OTHER participant (not the current user). Identity is hydrated via
// userclient since the repo no longer joins on the user table.
func (s *ChatService) GetNavContact(ctx context.Context, userID int) ([]dto.NavContactItem, *errors.AppError) {
	rooms, err := s.chatRepo.FindRoomsForUser(userID)
	if err != nil {
		return nil, errors.ErrInternal("查询聊天室失败")
	}
	if len(rooms) == 0 {
		return []dto.NavContactItem{}, nil
	}

	roomIDs := make([]int, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}

	// Participants: { room_id -> [participant...] }
	participants := s.chatRepo.FindParticipantsByRoomIDs(roomIDs)
	roomParts := make(map[int][]repository.ParticipantRow)
	for _, p := range participants {
		roomParts[p.ChatRoomID] = append(roomParts[p.ChatRoomID], p)
	}

	// Hydrate participant identity in one batch.
	pids := userclient.CollectIDs(participants, func(p repository.ParticipantRow) int { return p.UserID })
	userMap := s.userClient.Hydrate(ctx, pids)

	// Unread + total counts per room.
	unreadMap := make(map[int]int)
	for _, u := range s.chatRepo.CountUnreadByRoomIDs(roomIDs, userID) {
		unreadMap[u.ChatRoomID] = u.Count
	}
	totalMap := make(map[int]int)
	for _, t := range s.chatRepo.CountTotalByRoomIDs(roomIDs) {
		totalMap[t.ChatRoomID] = t.Count
	}

	items := make([]dto.NavContactItem, len(rooms))
	for i, r := range rooms {
		title, avatar, route := r.Name, r.Avatar, r.Name
		if r.Type == "private" {
			for _, p := range roomParts[r.ID] {
				if p.UserID != userID {
					u := userMap[p.UserID]
					title = u.Name
					avatar = u.Avatar
					route = strconv.Itoa(p.UserID)
					break
				}
			}
		}
		items[i] = dto.NavContactItem{
			ChatroomName:    r.Name,
			Content:         r.LastMessageContent,
			LastMessageTime: r.LastMessageTime,
			Count:           totalMap[r.ID],
			UnreadCount:     unreadMap[r.ID],
			Route:           route,
			Title:           title,
			Avatar:          avatar,
		}
	}
	return items, nil
}

// ──────────────────────────────────────────
// GetChatHistory — GET /api/message/chat/history
// ──────────────────────────────────────────

// GetChatHistory returns paginated chat messages between the current user and
// the receiver, in chronological (ascending) order. Side effect: marks every
// fetched message not sent by the current user as read.
func (s *ChatService) GetChatHistory(
	ctx context.Context,
	userID int,
	req *dto.GetChatHistoryRequest,
) ([]dto.ChatMessageItem, *errors.AppError) {
	if req.ReceiverID == userID {
		return nil, errors.ErrBadRequest("不能给自己发送消息")
	}

	roomID, roomName, err := s.findOrCreatePrivateRoom(userID, req.ReceiverID)
	if err != nil {
		return nil, errors.ErrInternal("查询聊天室失败")
	}
	if roomID == 0 {
		return []dto.ChatMessageItem{}, nil
	}

	rows := s.chatRepo.FindMessagesByRoom(roomID, roomName, req.Page, req.Limit)

	// Mark messages received (not sent by current user) as read. Read-receipts
	// are non-critical — log on failure but still return the history.
	if len(rows) > 0 {
		msgIDs := make([]int, 0, len(rows))
		for _, m := range rows {
			if m.SenderID != userID {
				msgIDs = append(msgIDs, m.ID)
			}
		}
		if err := s.chatRepo.MarkMessagesRead(msgIDs, userID); err != nil {
			slog.Warn("标记聊天消息已读失败", "user_id", userID, "room_id", roomID, "error", err)
		}
	}

	// Hydrate sender identity in one batch.
	uids := userclient.CollectIDs(rows, func(m repository.ChatMessageRow) int { return m.SenderID })
	userMap := s.userClient.Hydrate(ctx, uids)

	// Reverse DB order (DESC) into chronological ASC order for the response.
	items := make([]dto.ChatMessageItem, len(rows))
	for i, m := range rows {
		u := userMap[m.SenderID]

		// Recalled messages must not leak their original body — the client only
		// ever shows "<sender> 撤回了一条消息", never the content. Blank both the
		// raw source and the rendered HTML so a recalled message's text (and any
		// image URL) isn't served at all. (The image bytes themselves still live
		// on the CDN — true deletion is a separate, larger change.)
		content := m.Content
		contentHTML := markdown.RenderInline(m.Content)
		if m.IsRecall {
			content = ""
			contentHTML = ""
		}

		items[len(rows)-1-i] = dto.ChatMessageItem{
			ID:           m.ID,
			ChatroomName: m.ChatroomName,
			Sender:       dto.ChatSender{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			ReceiverID:  m.ReceiverID,
			Content:      content,
			ContentHtml:  contentHTML,
			IsRecall:     m.IsRecall,
			Created:      m.Created,
			RecallTime:   m.RecallTime,
			EditTime:     m.EditTime,
			ReadBy:       []dto.ChatSender{},
		}
	}
	return items, nil
}

// ──────────────────────────────────────────
// SendChatMessage — POST /api/message/chat/send
// ──────────────────────────────────────────

// SendChatMessage writes a new chat message and updates the room's
// last_message_* fields. senderName is written into chat_room for display in
// the contacts list; it's passed in because the service doesn't have a user repo.
func (s *ChatService) SendChatMessage(
	ctx context.Context,
	senderUserID int,
	senderName string,
	req *dto.SendChatMessageRequest,
) *errors.AppError {
	if req.ReceiverID == senderUserID {
		return errors.ErrBadRequest("不能给自己发送消息")
	}

	roomID, roomName, err := s.findOrCreatePrivateRoom(senderUserID, req.ReceiverID)
	if err != nil || roomID == 0 {
		return errors.ErrInternal("创建聊天室失败")
	}

	now := time.Now()
	// Atomic: the message insert and the room-preview update either both land
	// or neither does. Surface the error instead of a false "发送成功".
	txErr := s.chatRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.chatRepo.InsertChatMessage(tx, roomID, roomName, senderUserID, req.ReceiverID, req.Content, now); err != nil {
			return err
		}
		return s.chatRepo.UpdateRoomLastMessage(tx, roomID, req.Content, senderUserID, senderName, now)
	})
	if txErr != nil {
		return errors.ErrInternal("发送消息失败")
	}
	return nil
}

// ──────────────────────────────────────────
// RecallMessage — POST /api/message/chat/recall
// ──────────────────────────────────────────

// RecallMessage marks a chat message as recalled. Only the original sender
// may recall, and only if it hasn't been recalled already. When the recalled
// message was the room's latest, the room's last_message preview is also
// refreshed to "<sender>撤回了一条消息" so contact-list rendering matches the
// chat history view.
func (s *ChatService) RecallMessage(
	ctx context.Context,
	userID int,
	messageID int,
) *errors.AppError {
	header, ok := s.chatRepo.FindMessageHeader(messageID)
	if !ok {
		return errors.ErrNotFound("消息不存在或已被删除")
	}
	if header.SenderID != userID {
		return errors.ErrForbidden("您只能撤回自己发送的消息")
	}
	if header.IsRecall {
		return errors.ErrBadRequest("该消息已被撤回")
	}

	now := time.Now()
	if err := s.chatRepo.MarkMessageRecalled(messageID, now); err != nil {
		return errors.ErrInternal("撤回消息失败")
	}

	// Only refresh the room preview if this WAS the latest message —
	// otherwise the preview should keep showing whatever's actually latest.
	if s.chatRepo.IsLatestMessageInRoom(header.ChatRoomID, messageID) {
		// Look up the sender's current name from OAuth for the preview text.
		senderName := ""
		if u, _, err := s.userClient.User(ctx, header.SenderID); err == nil {
			senderName = u.Name
		}
		preview := fmt.Sprintf("%s撤回了一条消息", senderName)
		if err := s.chatRepo.UpdateRoomLastMessage(s.chatRepo.DB(), header.ChatRoomID, preview, userID, senderName, now); err != nil {
			slog.Warn("撤回后刷新聊天室预览失败", "room_id", header.ChatRoomID, "error", err)
		}
	}
	return nil
}

// ──────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────

// findOrCreatePrivateRoom returns the (roomID, roomName) for the private room
// between two users, creating it if it doesn't exist. Look-up is by
// participant table (NOT by generated room name, which may be stale after the
// OAuth migration changed user IDs).
func (s *ChatService) findOrCreatePrivateRoom(uid1, uid2 int) (int, string, error) {
	room := s.chatRepo.FindPrivateRoomBetween(uid1, uid2)
	if room.ID > 0 {
		return room.ID, room.Name, nil
	}
	newRoom, err := s.chatRepo.CreatePrivateRoom(generateRoomID(uid1, uid2), uid1, uid2)
	if err != nil {
		return 0, "", err
	}
	return newRoom.ID, newRoom.Name, nil
}

// generateRoomID produces a deterministic "smaller-larger" name for a private
// room between two users.
func generateRoomID(uid1, uid2 int) string {
	if uid1 < uid2 {
		return fmt.Sprintf("%d-%d", uid1, uid2)
	}
	return fmt.Sprintf("%d-%d", uid2, uid1)
}
