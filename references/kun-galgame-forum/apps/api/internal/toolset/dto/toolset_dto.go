package dto

import (
	"encoding/json"
	"time"

	"kun-galgame-api/internal/toolset/model"
	userModel "kun-galgame-api/internal/user/model"
)

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type ToolsetListRequest struct {
	Page      int    `query:"page" validate:"min=1"`
	Limit     int    `query:"limit" validate:"min=1,max=100"`
	Type      string `query:"type"`
	Language  string `query:"language"`
	Platform  string `query:"platform"`
	Version   string `query:"version"`
	SortField string `query:"sort_field"`
	SortOrder string `query:"sort_order"`
	// UserID is set by the per-user handler (GET /user/:id/toolsets) from the
	// path, NOT bound from the query — so the public /toolset list can't be
	// author-filtered by an arbitrary caller.
	UserID int `query:"-"`
}

type CreateToolsetRequest struct {
	Name        string   `json:"name" validate:"required,max=500"`
	Description string   `json:"description" validate:"max=2000"`
	Type        string   `json:"type"`
	Language    string   `json:"language"`
	Platform    string   `json:"platform"`
	Homepage    []string `json:"homepage"`
	Version     string   `json:"version" validate:"max=233"`
	Aliases     []string `json:"aliases"`
}

type UpdateToolsetRequest struct {
	Name        string   `json:"name" validate:"required,max=500"`
	Description string   `json:"description" validate:"max=2000"`
	Type        string   `json:"type"`
	Language    string   `json:"language"`
	Platform    string   `json:"platform"`
	Homepage    []string `json:"homepage"`
	Version     string   `json:"version" validate:"max=233"`
	Aliases     []string `json:"aliases"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

// ToolsetCard is the shape of each item in the list response.
//
// Note: `PracticalityAvg` is `any` so we can emit `null` when there are no
// ratings (the original handler did the same).
type ToolsetCard struct {
	ID                 int                 `json:"id"`
	Name               string              `json:"name"`
	User               userModel.UserBrief `json:"user"`
	Type               string              `json:"type"`
	Platform           string              `json:"platform"`
	Language           string              `json:"language"`
	Version            string              `json:"version"`
	View               int                 `json:"view"`
	Download           int                 `json:"download"`
	CommentCount       int                 `json:"comment_count"`
	PracticalityAvg    any                 `json:"practicality_avg"`
	ResourceUpdateTime any                 `json:"resource_update_time"`
}

// ToolsetResourceItem is the slim resource projection embedded in the toolset
// detail response. Hides sensitive fields (code/password/note) — those are
// only served by the dedicated /toolset/:id/resource/detail endpoint.
type ToolsetResourceItem struct {
	ID       int    `json:"id"`
	Type     string `json:"type"`
	Size     string `json:"size"`
	Download int    `json:"download"`
	Status   int    `json:"status"`
}

// ToolsetDetailResponse is the FLAT response for GET /toolset/:id, shaped to
// match the frontend `ToolsetDetail` type directly. Previously this was a
// nested envelope (`{toolset, descriptionHTML, ...}`) which made the
// frontend look up `data.toolset.created` — but the page reads `data.created`,
// causing `new Date(undefined).toISOString()` to throw RangeError.
type ToolsetDetailResponse struct {
	ID                 int                           `json:"id"`
	Name               string                        `json:"name"`
	ContentMarkdown    string                        `json:"content_markdown"`
	ContentHTML        string                        `json:"content_html"`
	Type               string                        `json:"type"`
	Platform           string                        `json:"platform"`
	Language           string                        `json:"language"`
	Version            string                        `json:"version"`
	Homepage           []string                      `json:"homepage"`
	View               int                           `json:"view"`
	Download           int64                         `json:"download"`
	User               userModel.UserBrief           `json:"user"`
	Aliases            []string                      `json:"aliases"`
	PracticalityAvg    *float64                      `json:"practicality_avg"`
	PracticalityCount  int64                         `json:"practicality_count"`
	RatingCounts       map[int]int64                 `json:"rating_counts"`
	ResourceUpdateTime time.Time                     `json:"resource_update_time"`
	Resource           []ToolsetResourceItem         `json:"resource"`
	Edited             *time.Time                    `json:"edited"`
	Created            time.Time                     `json:"created"`
	Updated            time.Time                     `json:"updated"`
	CommentCount       int64                         `json:"comment_count"`
	CommentPreview     []CommentDetailItem           `json:"comment_preview"`
	Contributors       []userModel.UserBrief         `json:"contributors"`
}

// CreatedToolsetResponse is the raw toolset row returned by POST /toolset.
type CreatedToolsetResponse = model.GalgameToolset

// HomepageJSON is a convenience alias used by the service when encoding homepage.
type HomepageJSON = json.RawMessage
