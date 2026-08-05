package model

import "time"

// GalgamePostLike is kungal's LOCAL like row for a community post (charter
// ruling 8: the community PostView carries no reaction count, so the display
// count + "我赞过" flag live here). It mirrors the community reaction toggle —
// the BFF drives this table from the primitive's authoritative added/removed
// result — while the community reaction itself feeds the trust engine. Distinct
// from the frozen legacy galgame_comment_like (which FK-pins old comment ids).
//
// Created by migration 057. PostID is the community post id (bigint).
type GalgamePostLike struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PostID    int64     `gorm:"column:post_id;not null;uniqueIndex:idx_galgame_post_like" json:"post_id"`
	UserID    int       `gorm:"column:user_id;not null;uniqueIndex:idx_galgame_post_like" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created;autoCreateTime" json:"created"`
}

func (GalgamePostLike) TableName() string { return "galgame_post_like" }

// GalgameCommentCommunityMap resolves an OLD galgame_comment id to its migrated
// community (thread, post) so kungal keeps deep-link / old-anchor continuity
// (charter ruling 9). Written by the infra import tool (step 02) and adopted
// idempotently by migration 057 with the SAME DDL (both CREATE TABLE IF NOT
// EXISTS). Read-only from the BFF (the locate endpoint).
type GalgameCommentCommunityMap struct {
	OldCommentID int   `gorm:"column:old_comment_id;primaryKey" json:"old_comment_id"`
	ThreadID     int64 `gorm:"column:thread_id;not null" json:"thread_id"`
	PostID       int64 `gorm:"column:post_id;not null" json:"post_id"`
	GalgameID    int   `gorm:"column:galgame_id;not null" json:"galgame_id"`
}

func (GalgameCommentCommunityMap) TableName() string { return "galgame_comment_community_map" }
