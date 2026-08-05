package dto

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// GetChatHistoryRequest is the query for GET /api/message/chat/history.
type GetChatHistoryRequest struct {
	ReceiverID int `query:"receiver_id" validate:"required,min=1"`
	Page        int `query:"page" validate:"min=1"`
	Limit       int `query:"limit" validate:"min=1,max=50"`
}

// SendChatMessageRequest is the body for POST /api/message/chat/send.
type SendChatMessageRequest struct {
	ReceiverID int    `json:"receiver_id" validate:"required,min=1"`
	Content     string `json:"content" validate:"required,min=1,max=1000"`
}

// RecallChatMessageRequest is the body for POST /api/message/chat/recall.
type RecallChatMessageRequest struct {
	MessageID int `json:"message_id" validate:"required,min=1"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

// ChatSender is the sender object embedded in chat messages.
type ChatSender struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// ChatMessageItem is a single chat message item returned by GetChatHistory.
// Field names/shape must match the frontend exactly.
//
// Content is the raw markdown source (kept for editing/quoting); ContentHtml is
// the server-rendered, sanitized inline+image HTML the client renders with
// v-html. Rendering happens server-side because the client never sanitizes
// (see internal/infrastructure/markdown).
type ChatMessageItem struct {
	ID           int         `json:"id"`
	ChatroomName string      `json:"chatroom_name"`
	Sender       ChatSender  `json:"sender"`
	ReceiverID  int         `json:"receiver_id"`
	Content      string      `json:"content"`
	ContentHtml  string      `json:"content_html"`
	IsRecall     bool        `json:"is_recall"`
	Created      string      `json:"created"`
	RecallTime   *string     `json:"recall_time"`
	EditTime     *string     `json:"edit_time"`
	ReadBy       []ChatSender `json:"read_by"`
}

// NavContactItem is a chat room entry for the message sidebar.
// Field names/shape must match the frontend exactly.
type NavContactItem struct {
	ChatroomName    string  `json:"chatroom_name"`
	Content         string  `json:"content"`
	LastMessageTime *string `json:"last_message_time"`
	Count           int     `json:"count"`
	UnreadCount     int     `json:"unread_count"`
	Route           string  `json:"route"`
	Title           string  `json:"title"`
	Avatar          string  `json:"avatar"`
}
