package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────
// Website core
// ──────────────────────────────────────────

type GalgameWebsite struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	URL         string `gorm:"column:url;uniqueIndex;not null" json:"url"`
	CreateTime  string `gorm:"column:create_time;not null" json:"create_time"`
	Description string `gorm:"default:''" json:"description"`
	Icon        string `gorm:"default:''" json:"icon"`
	// IconImageHash is the content-addressed image_service hash, preferred over
	// the legacy Icon URL (kept as fallback).
	IconImageHash string          `gorm:"column:icon_image_hash;default:''" json:"icon_image_hash"`
	View          int             `gorm:"default:0" json:"view"`
	Language      string          `gorm:"default:'JA'" json:"language"`
	AgeLimit      string          `gorm:"column:age_limit;default:'all'" json:"age_limit"` // all, r18
	Domain        json.RawMessage `gorm:"type:jsonb;default:'[]'" json:"domain"`           // text[] → jsonb

	// galgame_website.category_id references galgame_website_category(id)
	// with `ON DELETE RESTRICT` at the DB level (see 000_baseline.up.sql).
	// Deleting a category while websites exist will fail with a
	// foreign_key_violation — categories are taxonomy and should not
	// silently take their websites with them. Move/delete websites first.
	// (`constraint:OnDelete:RESTRICT` is a doc tag only — GORM only acts
	//  on it under AutoMigrate, which this project doesn't run.)
	CategoryID int `gorm:"column:category_id;not null;constraint:OnDelete:RESTRICT" json:"category_id"`
	UserID     int `gorm:"column:user_id;not null;default:2" json:"user_id"`

	// Counts (denormalized)
	LikeCount     int `gorm:"column:like_count;default:0" json:"like_count"`
	FavoriteCount int `gorm:"column:favorite_count;default:0" json:"favorite_count"`
	CommentCount  int `gorm:"column:comment_count;default:0" json:"comment_count"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsite) TableName() string { return "galgame_website" }

// ──────────────────────────────────────────
// Category & Tag
// ──────────────────────────────────────────

type GalgameWebsiteCategory struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"uniqueIndex;not null" json:"name"` // resource, patch, community, telegram, other
	Label       string `gorm:"default:''" json:"label"`
	Description string `gorm:"default:''" json:"description"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsiteCategory) TableName() string { return "galgame_website_category" }

type GalgameWebsiteTag struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Level       int    `gorm:"not null" json:"level"`
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Label       string `gorm:"default:''" json:"label"`
	Description string `gorm:"default:''" json:"description"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsiteTag) TableName() string { return "galgame_website_tag" }

type GalgameWebsiteTagRelation struct {
	GalgameWebsiteID    int `gorm:"column:galgame_website_id;primaryKey" json:"galgame_website_id"`
	GalgameWebsiteTagID int `gorm:"column:galgame_website_tag_id;primaryKey" json:"galgame_website_tag_id"`

	Tag GalgameWebsiteTag `gorm:"foreignKey:GalgameWebsiteTagID;constraint:OnDelete:CASCADE" json:"tag,omitzero"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsiteTagRelation) TableName() string { return "galgame_website_tag_relation" }

// The legacy galgame_website_comment model was retired in charter step 06a
// (comments moved to the infra community primitive) and dropped by migration 060.

// ──────────────────────────────────────────
// Like & Favorite (composite primary keys)
// ──────────────────────────────────────────

type GalgameWebsiteLike struct {
	UserID    int `gorm:"column:user_id;primaryKey" json:"user_id"`
	WebsiteID int `gorm:"column:website_id;primaryKey" json:"website_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsiteLike) TableName() string { return "galgame_website_like" }

type GalgameWebsiteFavorite struct {
	UserID    int `gorm:"column:user_id;primaryKey" json:"user_id"`
	WebsiteID int `gorm:"column:website_id;primaryKey" json:"website_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameWebsiteFavorite) TableName() string { return "galgame_website_favorite" }
