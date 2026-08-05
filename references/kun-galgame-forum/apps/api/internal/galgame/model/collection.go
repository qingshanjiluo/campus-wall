package model

import "time"

// ──────────────────────────────────────────
// Collections (收藏夹) — user-owned, many-to-many over galgames.
// Replaces the flat galgame_favorite list (migration 043). A galgame can live
// in many of a user's collections; each collection carries a privacy setting.
// ──────────────────────────────────────────

// Collection visibility values (stored in galgame_collection.visibility).
const (
	CollectionPublic     = "public"     // anyone
	CollectionPrivate    = "private"    // owner only
	CollectionRestricted = "restricted" // owner + galgame_collection_viewer allow-list
)

type GalgameCollection struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int    `gorm:"column:user_id;not null" json:"user_id"`
	Name        string `gorm:"column:name;type:varchar(60);not null" json:"name"`
	Description string `gorm:"column:description;type:varchar(500);not null;default:''" json:"description"`
	Visibility  string `gorm:"column:visibility;type:varchar(16);not null;default:'public'" json:"visibility"`
	IsDefault   bool   `gorm:"column:is_default;not null;default:false" json:"is_default"`
	ItemCount   int    `gorm:"column:item_count;not null;default:0" json:"item_count"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameCollection) TableName() string { return "galgame_collection" }

type GalgameCollectionItem struct {
	ID           int `gorm:"primaryKey;autoIncrement" json:"id"`
	CollectionID int `gorm:"column:collection_id;not null;uniqueIndex:idx_gci_unique" json:"collection_id"`
	GalgameID    int `gorm:"column:galgame_id;not null;uniqueIndex:idx_gci_unique" json:"galgame_id"`
	// UserID is the collection owner, denormalized so "is this game in any of my
	// collections" + the distinct-user favorite_count run index-only (idx_gci_user_galgame).
	// A collection's owner never changes, so this carries no consistency risk.
	UserID int `gorm:"column:user_id;not null" json:"user_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameCollectionItem) TableName() string { return "galgame_collection_item" }

type GalgameCollectionViewer struct {
	ID           int `gorm:"primaryKey;autoIncrement" json:"id"`
	CollectionID int `gorm:"column:collection_id;not null;uniqueIndex:idx_gcv_unique" json:"collection_id"`
	UserID       int `gorm:"column:user_id;not null;uniqueIndex:idx_gcv_unique" json:"user_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
}

func (GalgameCollectionViewer) TableName() string { return "galgame_collection_viewer" }
