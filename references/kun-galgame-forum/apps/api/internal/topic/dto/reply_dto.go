package dto

import "time"

// ──────────────────────────────────────────
// Reply requests
// ──────────────────────────────────────────

type ListRepliesRequest struct {
	TopicID   int    `query:"topic_id" validate:"required,min=1"`
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=30"`
	SortOrder string `query:"sort_order" validate:"required,oneof=asc desc"`
}

// ReplyLocateResponse tells the frontend which reply-stream page a deep-link
// target (a reply floor or a comment id) lives on, so it can load that page
// directly and scroll to the target. ReplyID is set for a comment target (the
// reply whose comment panel must open); CommentID echoes the requested comment.
type ReplyLocateResponse struct {
	Page      int `json:"page"`
	Floor     int `json:"floor"`
	ReplyID   int `json:"reply_id"`
	CommentID int `json:"comment_id"`
}

// Multi-target replies were retired: a reply now carries one body with inline
// @mention / #quote tokens. The Phase-4 migration folded all legacy
// topic_reply_target rows into Content, so the read-side target response is gone.

type CreateReplyRequest struct {
	TopicID int    `json:"topic_id" validate:"required,min=1"`
	Content string `json:"content" validate:"required,max=10007"`
}

type UpdateReplyRequest struct {
	ReplyID int    `json:"reply_id" validate:"required,min=1"`
	Content string `json:"content" validate:"required,max=10007"`
}

type ReplyInteractionRequest struct {
	ReplyID int `json:"reply_id" validate:"required,min=1"`
}

// ReactionRequest is the body for PUT /topic/:tid/reaction.
type ReactionRequest struct {
	Reaction string `json:"reaction" validate:"required"`
}

// ReplyReactionRequest is the body for PUT /topic/:tid/reply/reaction.
type ReplyReactionRequest struct {
	ReplyID  int    `json:"reply_id" validate:"required,min=1"`
	Reaction string `json:"reaction" validate:"required"`
}

type BestAnswerRequest struct {
	TopicID int `json:"topic_id" validate:"required,min=1"`
	ReplyID int `json:"reply_id" validate:"required,min=1"`
}

type PinReplyRequest struct {
	TopicID int `json:"topic_id" validate:"required,min=1"`
	ReplyID int `json:"reply_id" validate:"required,min=1"`
}

// ──────────────────────────────────────────
// Reply responses
// ──────────────────────────────────────────

type TopicReplyResponse struct {
	ID              int                    `json:"id"`
	TopicID         int                    `json:"topic_id"`
	Floor           int                    `json:"floor"`
	User            KunUserWithMoemoepoint `json:"user"`
	Edited          *time.Time             `json:"edited"`
	ContentMarkdown string                 `json:"content_markdown"`
	ContentHtml     string                 `json:"content_html"`
	LikeCount       int                    `json:"like_count"`
	IsLiked         bool                   `json:"is_liked"`
	DislikeCount    int                    `json:"dislike_count"`
	IsDisliked      bool                   `json:"is_disliked"`
	Reactions       []ReactionSummary      `json:"reactions"`
	Comments        []TopicCommentResponse `json:"comment"`
	IsPinned        bool                   `json:"is_pinned"`
	IsBestAnswer    bool                   `json:"is_best_answer"`
	Created         time.Time              `json:"created"`
}

// ──────────────────────────────────────────
// Comment requests
// ──────────────────────────────────────────

type CreateCommentRequest struct {
	TopicID      int    `json:"topic_id" validate:"required,min=1"`
	ReplyID      int    `json:"reply_id" validate:"required,min=1"`
	TargetUserID int    `json:"target_user_id" validate:"required,min=1"`
	Content      string `json:"content" validate:"required,min=1,max=1007"`
	// ParentCommentID is set when replying to another comment (nested); omitted
	// for a top-level comment on the reply. The service validates it points at a
	// comment on the same reply.
	ParentCommentID *int `json:"parent_comment_id" validate:"omitempty,min=1"`
}

type CommentInteractionRequest struct {
	CommentID int `json:"comment_id" validate:"required,min=1"`
}

type UpdateCommentRequest struct {
	CommentID int    `json:"comment_id" validate:"required,min=1"`
	Content   string `json:"content" validate:"required,min=1,max=1007"`
}

// ──────────────────────────────────────────
// Comment responses
// ──────────────────────────────────────────

type TopicCommentResponse struct {
	ID         int     `json:"id"`
	ReplyID    int     `json:"reply_id"`
	TopicID    int     `json:"topic_id"`
	User       KunUser `json:"user"`
	TargetUser KunUser `json:"target_user"`
	// ParentCommentID is the comment this one replies to (nil = top-level).
	ParentCommentID *int       `json:"parent_comment_id"`
	Content         string     `json:"content"`
	IsLiked         bool       `json:"is_liked"`
	LikeCount       int        `json:"like_count"`
	Created         time.Time  `json:"created"`
	Edited          *time.Time `json:"edited"`
}
