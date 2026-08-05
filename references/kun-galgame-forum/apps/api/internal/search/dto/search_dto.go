package dto

import "time"

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type SearchRequest struct {
	Keywords string `query:"keywords" validate:"required,max=107"`
	Type     string `query:"type" validate:"required,oneof=topic galgame user reply comment"`
	Page     int    `query:"page" validate:"min=1"`
	Limit    int    `query:"limit" validate:"min=1,max=12"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type UserBrief struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// TopicItem mirrors home.dto.HomeTopic so the FE renders search-topic
// results through the same HomeTopicCard (badges, section chips,
// poll/best-answer/NSFW flags). Without the extra fields the card
// silently dropped the badges and Vue v-for warned on undefined
// section arrays.
type TopicItem struct {
	ID               int        `json:"id"`
	Title            string     `json:"title"`
	View             int        `json:"view"`
	Status           int        `json:"status"`
	LikeCount        int        `json:"like_count"`
	ReplyCount       int        `json:"reply_count"`
	CommentCount     int        `json:"comment_count"`
	HasBestAnswer    bool       `json:"has_best_answer"`
	IsPollTopic      bool       `json:"is_poll_topic"`
	IsNSFWTopic      bool       `json:"is_nsfw_topic"`
	Section          []string   `json:"section"`
	UpvoteTime       *time.Time `json:"upvote_time"`
	StatusUpdateTime time.Time  `json:"status_update_time"`
	User             UserBrief  `json:"user"`
}

type UserItem struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Avatar      string    `json:"avatar"`
	Bio         string    `json:"bio"`
	Moemoepoint int       `json:"moemoepoint"`
	Created     time.Time `json:"created"`
}

type ReplyItem struct {
	ID         int       `json:"id"`
	TopicID    int       `json:"topic_id"`
	TopicTitle string    `json:"topic_title"`
	Content    string    `json:"content"`
	Floor      int       `json:"floor"`
	User       UserBrief `json:"user"`
	Created    time.Time `json:"created"`
}

type CommentItem struct {
	ID         int       `json:"id"`
	TopicID    int       `json:"topic_id"`
	TopicTitle string    `json:"topic_title"`
	Content    string    `json:"content"`
	User       UserBrief `json:"user"`
	Created    time.Time `json:"created"`
}

// PaginatedResult is a generic paginated payload returned by service methods.
type PaginatedResult[T any] struct {
	Items []T
	Total int64
}
