package dto

import (
	"time"

	userModel "kun-galgame-api/internal/user/model"
)

// CommentDetailItem is a slim comment + user projection used by the toolset
// detail response (commentPreview field). It is sourced from the community
// primitive (charter step 06a); the frozen galgame_toolset_comment table and its
// full-CRUD wire shapes were retired (migration 060).
//
// Explicit camelCase fields keep the FE ToolsetComment type contract on
// /toolset/:id consistent with /toolset/:id/comment/all.
type CommentDetailItem struct {
	ID        int                 `json:"id"`
	Content   string              `json:"content"`
	UserID    int                 `json:"user_id"`
	ToolsetID int                 `json:"toolset_id"`
	ParentID  *int                `json:"parent_id"`
	Edited    *time.Time          `json:"edited"`
	Created   time.Time           `json:"created"`
	Updated   time.Time           `json:"updated"`
	User      userModel.UserBrief `json:"user"`
}
