package dto

import "time"

// SaveTopicDraftRequest is the body for POST /topic/draft — a snapshot of the
// editor. Everything is optional (a draft is work-in-progress); the service
// rejects a draft that is entirely empty. Field names mirror CreateTopicRequest
// (section) so the same editor state serializes for both.
type SaveTopicDraftRequest struct {
	Title       string   `json:"title" validate:"max=233"`
	Content     string   `json:"content" validate:"max=100007"`
	Category    string   `json:"category" validate:"omitempty,oneof=galgame technique others"`
	Sections    []string `json:"section" validate:"omitempty,max=3"`
	IsNSFW      bool     `json:"is_nsfw"`
	CoverImages []string `json:"cover_images" validate:"omitempty,max=9"`
}

// TopicDraftListItem is one row in GET /topic/draft — metadata only (the full
// content is fetched on load), plus a short raw-markdown summary so a title-less
// draft still shows something.
type TopicDraftListItem struct {
	ID      int       `json:"id"`
	Title   string    `json:"title"`
	Summary string    `json:"summary"`
	Updated time.Time `json:"updated"`
}

// TopicDraftDetail is GET /topic/draft/:id — the full snapshot restored into the
// editor. Field names mirror the create-topic request (section).
type TopicDraftDetail struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Category    string    `json:"category"`
	Sections    []string  `json:"section"`
	IsNSFW      bool      `json:"is_nsfw"`
	CoverImages []string  `json:"cover_images"`
	Updated     time.Time `json:"updated"`
}
