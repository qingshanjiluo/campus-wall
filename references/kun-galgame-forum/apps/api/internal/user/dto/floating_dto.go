package dto

// FloatingCardRequest binds the query params for GET /api/user/:userID/floating.
// NOTE: the URL path parameter :userID is historically ignored — the frontend
// passes the target user via ?userId=N. Preserve this quirk for compatibility.
type FloatingCardRequest struct {
	UserID int `query:"user_id" validate:"required,min=1"`
}

// FloatingCardResponse is the hover-card payload returned to the frontend.
type FloatingCardResponse struct {
	ID                   int    `json:"id"`
	Name                 string `json:"name"`
	Avatar               string `json:"avatar"`
	Moemoepoint          int    `json:"moemoepoint"`
	TopicCount           int64  `json:"topic_count"`
	TopicReplyCount      int64  `json:"topic_reply_count"`
	TopicCommentCount    int64  `json:"topic_comment_count"`
	GalgameResourceCount int64  `json:"galgame_resource_count"`
}
