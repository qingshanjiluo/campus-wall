package model

import "time"

// FriendLink is an admin-managed 友情链接 row — replaces the static frontend
// config apps/web/app/config/friend.json. Grouped by Category (one of
// official / galgame / others); SortOrder is the drag-reorder position WITHIN a
// category (ascending). Banner is a full image URL (image_service webp for new
// uploads; seeded rows keep the /friends/<name>.webp static path).
type FriendLink struct {
	ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Category    string `gorm:"not null" json:"category"`
	Name        string `gorm:"not null" json:"name"`
	Link        string `gorm:"not null" json:"link"`
	Description string `gorm:"type:text;default:''" json:"description"`
	Banner      string `gorm:"default:''" json:"banner"`
	// BannerImageHash is the content-addressed image_service hash, preferred over
	// the legacy Banner URL. BannerURL is the resolved CDN url (not persisted),
	// set on read so the FE never sees a raw hash.
	BannerImageHash string    `gorm:"column:banner_image_hash;default:''" json:"banner_image_hash"`
	BannerURL       string    `gorm:"-" json:"banner_url"`
	Status          string    `gorm:"default:'normal'" json:"status"`
	SortOrder       int       `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	CreatedAt       time.Time `gorm:"column:created" json:"created"`
	UpdatedAt       time.Time `gorm:"column:updated" json:"updated"`
}

func (FriendLink) TableName() string { return "friend_link" }
