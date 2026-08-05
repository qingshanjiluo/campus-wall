package repository

import (
	"kun-galgame-api/internal/rss/dto"

	"gorm.io/gorm"
)

type RSSRepository struct {
	db *gorm.DB
}

func NewRSSRepository(db *gorm.DB) *RSSRepository {
	return &RSSRepository{db: db}
}

// FindRecentSFWTopics returns the 10 most recent SFW topics for the RSS feed.
// Identity (UserName) is hydrated by the handler/service via userclient.
func (r *RSSRepository) FindRecentSFWTopics() []dto.TopicRSSItem {
	var topics []dto.TopicRSSItem
	r.db.Table("topic t").
		Select(`t.id, t.title, SUBSTRING(t.content, 1, 233) AS description,
			t.user_id, t.created`).
		Where("t.status != 1 AND t.is_nsfw = false").
		Order("t.created DESC").
		Limit(10).
		Find(&topics)
	return topics
}

// RecentGalgameRow is the local-only projection used to seed the galgame RSS:
// IDs and creation timestamp; metadata is fetched from galgame separately.
type RecentGalgameRow struct {
	ID      int    `gorm:"column:id"`
	Created string `gorm:"column:created"`
	// CreatorUserID is the frozen wiki-era submitter (migration 066) — the
	// feed author's only source, since the catalog face carries no submitter.
	// NULL = unknown, which the item renders with an empty author.
	CreatorUserID *int `gorm:"column:creator_user_id"`
}

// FindRecentGalgameIDs returns the most recent local galgame stub IDs, plus the
// frozen creator the feed's author field needs. Name and banner are resolved via
// the galgame client; the author is NOT — the catalog face carries none.
func (r *RSSRepository) FindRecentGalgameIDs(limit int) []RecentGalgameRow {
	var rows []RecentGalgameRow
	r.db.Table("galgame").
		Select("id, created, creator_user_id").
		Order("created DESC").
		Limit(limit).
		Scan(&rows)
	return rows
}
