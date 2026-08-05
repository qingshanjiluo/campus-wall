package dto

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type ListMessagesRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=30"`
	Type      string `query:"type"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type KunUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type MessageResponse struct {
	ID         int     `json:"id"`
	Sender     KunUser `json:"sender"`
	ReceiverID int     `json:"receiver_id"`
	Link       string  `json:"link"`
	Content    string  `json:"content"`
	Status     string  `json:"status"`
	Type       string  `json:"type"`
	// ISO timestamp from message.created; matches MessageRow.CreatedAt
	// string. See SystemMessageResponse.Created for the same fix —
	// the prior `time.Time` declaration was never populated by the
	// service.
	Created string `json:"created"`
}

type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	// `total` (not `totalCount`) to match the convention used elsewhere in
	// the codebase — every paginated list type. The previous `totalCount` tag
	// silently broke pagination on /message/notice (FE reads `data.value.total`).
	Total int64 `json:"total"`
}

type SystemMessageResponse struct {
	ID      int               `json:"id"`
	IsRead  bool              `json:"is_read"`
	Content map[string]string `json:"content"`
	Admin   KunUser           `json:"admin"`
	// ISO timestamp as returned by the DB — kept as string to match the
	// repo row type (system_message.created is selected straight into
	// `created_at string`). A previous `time.Time` declaration was never
	// assigned in the service and serialised as `0001-01-01T00:00:00Z`,
	// which the FE displayed as "约 2024 年前".
	Created string `json:"created"`
}
