package repository

import (
	"time"

	"gorm.io/gorm"
)

type HomeRepository struct {
	db *gorm.DB
}

func NewHomeRepository(db *gorm.DB) *HomeRepository {
	return &HomeRepository{db: db}
}

// ──────────────────────────────────────────
// Row projections
// ──────────────────────────────────────────

type GalgameLocalRow struct {
	ID                 int       `gorm:"column:id"`
	View               int       `gorm:"column:view"`
	LikeCount          int       `gorm:"column:like_count"`
	ResourceUpdateTime time.Time `gorm:"column:resource_update_time"`
	// CreatorUserID is the frozen wiki-era submitter (migration 066) — the
	// author chip's only source, since the catalog face carries no submitter.
	// NULL = unknown, which the card renders as no chip.
	CreatorUserID *int `gorm:"column:creator_user_id"`
}

type ResourcePLRow struct {
	GalgameID int    `gorm:"column:galgame_id"`
	Platform  string `gorm:"column:platform"`
	Language  string `gorm:"column:language"`
}

type TopicRow struct {
	ID               int        `gorm:"column:id"`
	Title            string     `gorm:"column:title"`
	View             int        `gorm:"column:view"`
	IsNSFW           bool       `gorm:"column:is_nsfw"`
	Status           int        `gorm:"column:status"`
	LikeCount        int        `gorm:"column:like_count"`
	ReplyCount       int        `gorm:"column:reply_count"`
	CommentCount     int        `gorm:"column:comment_count"`
	BestAnswerID     *int       `gorm:"column:best_answer_id"`
	UpvoteTime       *time.Time `gorm:"column:upvote_time"`
	StatusUpdateTime time.Time  `gorm:"column:status_update_time"`
	UserID           int        `gorm:"column:user_id"`
}

type SectionRelationRow struct {
	TopicID     int    `gorm:"column:topic_id"`
	SectionName string `gorm:"column:name"`
}

// ──────────────────────────────────────────
// Queries
// ──────────────────────────────────────────

// FindRecentGalgames returns the N galgames whose CONTENT was most recently
// updated locally — ordered by the dedicated resource_update_time column
// (migration 018), NOT the generic audit `updated` (which likes/comments bump).
func (r *HomeRepository) FindRecentGalgames(limit int) ([]GalgameLocalRow, error) {
	var rows []GalgameLocalRow
	err := r.db.Table("galgame").
		Select("id, view, like_count, resource_update_time, creator_user_id").
		Order("resource_update_time DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

// FindResourcePlatformLanguage returns (galgame_id, platform, language)
// rows for the given galgame IDs.
func (r *HomeRepository) FindResourcePlatformLanguage(galgameIDs []int) []ResourcePLRow {
	if len(galgameIDs) == 0 {
		return nil
	}
	var resources []ResourcePLRow
	r.db.Table("galgame_resource").
		Select("galgame_id, platform, language").
		Where("galgame_id IN ?", galgameIDs).
		Find(&resources)
	return resources
}

// FindHomeTopics returns the most recent topics (excluding some sections)
// updated within the last 3 months.
func (r *HomeRepository) FindHomeTopics(limit int, isSFW bool) ([]TopicRow, error) {
	threeMonthsAgo := time.Now().AddDate(0, -3, 0)
	excludedSections := []string{"g-seeking", "g-other", "t-help"}

	query := r.db.Table("topic").
		Select(`topic.id, topic.title, topic.view, topic.is_nsfw, topic.status,
			topic.like_count, topic.reply_count, topic.comment_count,
			topic.best_answer_id, topic.upvote_time, topic.status_update_time,
			topic.user_id`).
		Where("topic.status != 1").
		Where(`topic.id NOT IN (
			SELECT tsr.topic_id FROM topic_section_relation tsr
			JOIN topic_section ts ON ts.id = tsr.topic_section_id
			WHERE ts.name IN ?
		)`, excludedSections).
		Where(`(topic.edited >= ? OR (topic.edited IS NULL AND topic.created >= ?))`, threeMonthsAgo, threeMonthsAgo).
		Order("topic.status_update_time DESC").
		Limit(limit)

	if isSFW {
		query = query.Where("topic.is_nsfw = false")
	}

	var rows []TopicRow
	err := query.Find(&rows).Error
	return rows, err
}

// FindTopicSections returns section names grouped by topic ID.
func (r *HomeRepository) FindTopicSections(topicIDs []int) []SectionRelationRow {
	if len(topicIDs) == 0 {
		return nil
	}
	var sections []SectionRelationRow
	r.db.Table("topic_section_relation tsr").
		Select("tsr.topic_id, ts.name").
		Joins("JOIN topic_section ts ON ts.id = tsr.topic_section_id").
		Where("tsr.topic_id IN ?", topicIDs).
		Find(&sections)
	return sections
}

// FindTopicIDsWithPoll returns the subset of topicIDs that have at
// least one row in topic_poll. Mirrors
// topic_repo.FindTopicIDsWithPoll so the homepage feed can render the
// "投票" badge — it was hardcoded false on the home path even after
// the /topic + /resource list endpoints were batched.
func (r *HomeRepository) FindTopicIDsWithPoll(topicIDs []int) map[int]bool {
	out := map[int]bool{}
	if len(topicIDs) == 0 {
		return out
	}
	var rows []struct {
		TopicID int `gorm:"column:topic_id"`
	}
	r.db.Table("topic_poll").
		Where("topic_id IN ?", topicIDs).
		Select("topic_id").
		Scan(&rows)
	for _, row := range rows {
		out[row.TopicID] = true
	}
	return out
}
