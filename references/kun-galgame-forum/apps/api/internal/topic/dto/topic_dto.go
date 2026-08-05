package dto

import (
	"time"

	"kun-galgame-api/pkg/imageclient"
)

// ──────────────────────────────────────────
// Shared user projections
// ──────────────────────────────────────────

type KunUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type KunUserWithMoemoepoint struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	Moemoepoint int    `json:"moemoepoint"`
}

// TopicUpvoteRecord is one 推话题 record shown below a topic: who pushed it, their
// optional one-liner (may be empty — the FE shows a random default then), when.
type TopicUpvoteRecord struct {
	ID          int       `json:"id"`
	User        KunUser   `json:"user"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
}

// ReactionHistoryItem is one reaction event for the 查看历史 modal: who reacted,
// with which reaction key, and when. Newest first.
type ReactionHistoryItem struct {
	User     KunUser   `json:"user"`
	Reaction string    `json:"reaction"`
	Created  time.Time `json:"created"`
}

// ──────────────────────────────────────────
// Topic list
// ──────────────────────────────────────────

type ListTopicsRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=50"`
	SortField string `query:"sort_field"`
	SortOrder string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
	Category  string `query:"category"`
}

type TopicCard struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	View        int      `json:"view"`
	Sections    []string `json:"section"`
	CoverImages []string `json:"cover_images"`
	// Per-cover-token image metadata (dims + ThumbHash), keyed by the
	// /image/<hash> token in CoverImages — lets the FE reserve each cover's
	// aspect ratio (no CLS) and blur it up. Output-only; a token is absent when
	// image_service is unconfigured or its thumbhash backfill hasn't run.
	CoverImageMeta   map[string]imageclient.ImageMeta `json:"cover_image_meta,omitempty"`
	User             KunUser                          `json:"user"`
	Status           int                              `json:"status"`
	HasBestAnswer    bool                             `json:"has_best_answer"`
	IsPollTopic      bool                             `json:"is_poll_topic"`
	IsNSFW           bool                             `json:"is_nsfw_topic"`
	LikeCount        int                              `json:"like_count"`
	ReplyCount       int                              `json:"reply_count"`
	CommentCount     int                              `json:"comment_count"`
	StatusUpdateTime time.Time                        `json:"status_update_time"`
	Created          time.Time                        `json:"created"`
	UpvoteTime       *time.Time                       `json:"upvote_time"`
}

// TopicListResponse is the {topics, total} envelope for GET /topic — total drives
// the FE paginator.
type TopicListResponse struct {
	Topics []TopicCard `json:"topics"`
	Total  int64       `json:"total"`
}

// ──────────────────────────────────────────
// Topic detail
// ──────────────────────────────────────────

// ReactionSummary is one reaction key on a topic/reply: total count, whether the
// viewer reacted (`mine`), and — only when count < 5 — the reactors, so the FE
// shows avatars for small counts and just the emoji+count for large ones.
type ReactionSummary struct {
	Reaction string    `json:"reaction"`
	Count    int       `json:"count"`
	Mine     bool      `json:"mine"`
	Reactors []KunUser `json:"reactors,omitempty"`
}

// MyTopicInteractions is the current user's favorited topic ids + the reaction
// keys they hold per topic, returned by GET /topic/interactions/mine to hydrate
// feed-card 收藏 + reaction state (the shared feed cache can't carry per-user).
type MyTopicInteractions struct {
	Favorited []int            `json:"favorited"`
	Reactions map[int][]string `json:"reactions"`
}

type TopicDetail struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Content     string   `json:"content_markdown"`
	ContentHtml string   `json:"content_html"`
	View        int      `json:"view"`
	Status      int      `json:"status"`
	IsNSFW      bool     `json:"is_nsfw"`
	Category    string   `json:"category"`
	Sections    []string `json:"section"`
	CoverImages []string `json:"cover_images"`
	// See TopicCard.CoverImageMeta — keyed by the /image/<hash> cover token.
	CoverImageMeta   map[string]imageclient.ImageMeta `json:"cover_image_meta,omitempty"`
	User             KunUserWithMoemoepoint           `json:"user"`
	LikeCount        int                              `json:"like_count"`
	IsLiked          bool                             `json:"is_liked"`
	DislikeCount     int                              `json:"dislike_count"`
	IsDisliked       bool                             `json:"is_disliked"`
	FavoriteCount    int                              `json:"favorite_count"`
	IsFavorited      bool                             `json:"is_favorited"`
	UpvoteCount      int                              `json:"upvote_count"`
	IsUpvoted        bool                             `json:"is_upvoted"`
	Reactions        []ReactionSummary                `json:"reactions"`
	ReplyCount       int                              `json:"reply_count"`
	IsPollTopic      bool                             `json:"is_poll_topic"`
	StatusUpdateTime time.Time                        `json:"status_update_time"`
	UpvoteTime       *time.Time                       `json:"upvote_time"`
	Edited           *time.Time                       `json:"edited"`
	Created          time.Time                        `json:"created"`
	// Best answer summary — populated when topic.best_answer_id is set.
	// Embedded here (instead of forcing a second /reply fetch) so the
	// topic detail page can render JSON-LD `acceptedAnswer` schema during
	// SSR. nil = no best answer set.
	BestAnswer *TopicBestAnswer `json:"best_answer,omitempty"`
}

// TopicBestAnswer is the slim projection of the chosen reply that
// /topic/:tid embeds for SEO (schema.org acceptedAnswer).
type TopicBestAnswer struct {
	ID              int       `json:"id"`
	Floor           int       `json:"floor"`
	User            KunUser   `json:"user"`
	ContentMarkdown string    `json:"content_markdown"`
	ContentHtml     string    `json:"content_html"`
	Created         time.Time `json:"created"`
}

// ──────────────────────────────────────────
// Topic mutations
// ──────────────────────────────────────────

type CreateTopicRequest struct {
	Title       string   `json:"title" validate:"required,min=1,max=233"`
	Content     string   `json:"content" validate:"required,min=1,max=100007"`
	Category    string   `json:"category" validate:"required,oneof=galgame technique others"`
	Sections    []string `json:"section" validate:"required,min=1,max=3"`
	IsNSFW      bool     `json:"is_nsfw"`
	CoverImages []string `json:"cover_images" validate:"omitempty,max=9"`
}

type UpdateTopicRequest struct {
	Title       string   `json:"title" validate:"required,min=1,max=233"`
	Content     string   `json:"content" validate:"required,min=1,max=100007"`
	Category    string   `json:"category" validate:"required,oneof=galgame technique others"`
	Sections    []string `json:"section" validate:"required,min=1,max=3"`
	IsNSFW      bool     `json:"is_nsfw"`
	CoverImages []string `json:"cover_images" validate:"omitempty,max=9"`
}

type TopicInteractionRequest struct {
	TopicID int `json:"topic_id" validate:"required,min=1"`
}
