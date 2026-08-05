package repository

import (
	"time"

	"gorm.io/gorm"
)

type SearchRepository struct {
	db *gorm.DB
}

func NewSearchRepository(db *gorm.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

// ──────────────────────────────────────────
// Row projections
// ──────────────────────────────────────────

// TopicRow / ReplyRow / CommentRow no longer carry user identity; the service
// layer hydrates name/avatar via userclient since the user table is no longer
// the source of truth.
type TopicRow struct {
	ID               int
	Title            string
	View             int
	Status           int
	LikeCount        int
	ReplyCount       int
	CommentCount     int
	StatusUpdateTime time.Time
	UserID           int
	IsNSFW           bool
	BestAnswerID     *int
	UpvoteTime       *time.Time
}

// TopicSectionRow shares the shape used by home_repo: {topic_id, name}.
// Duplicated here rather than imported so the search module stays free of
// inter-module repo dependencies — the query is trivially small and the data
// shape won't drift independently.
type TopicSectionRow struct {
	TopicID     int    `gorm:"column:topic_id"`
	SectionName string `gorm:"column:name"`
}

type ReplyRow struct {
	ID          int
	TopicID     int
	TopicTitle  string
	TopicUserID int
	Content     string
	Floor       int
	UserID      int
	Created     time.Time
}

type CommentRow struct {
	ID          int
	TopicID     int
	TopicTitle  string
	TopicUserID int
	Content     string
	UserID      int
	Created     time.Time
}

// ──────────────────────────────────────────
// Queries
// ──────────────────────────────────────────

// SearchTopics fulltext-searches topics by title/content/category. Identity
// is hydrated by the service layer via userclient.
//
// Selects the same superset of fields the FE HomeTopicCard expects so a
// search-topic result renders with all the badges (best-answer / poll /
// NSFW / upvote chip) instead of silently missing them — the card is
// shared with the /home and /topic feeds.
func (r *SearchRepository) SearchTopics(keywords []string, page, limit int) (rows []TopicRow, total int64) {
	query := r.db.Table("topic t").
		Select(`t.id, t.title, t.view, t.status, t.like_count, t.reply_count,
			t.comment_count, t.status_update_time, t.user_id,
			t.is_nsfw, t.best_answer_id, t.upvote_time`).
		Where("t.status != 1")
	for _, kw := range keywords {
		like := "%" + kw + "%"
		query = query.Where("(t.title ILIKE ? OR t.content ILIKE ? OR t.category ILIKE ?)",
			like, like, like)
	}

	query.Count(&total)
	query.Order("t.status_update_time DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows)
	return
}

// FindTopicSections groups section names by topic id (same shape as
// home_repo.FindTopicSections).
func (r *SearchRepository) FindTopicSections(topicIDs []int) []TopicSectionRow {
	if len(topicIDs) == 0 {
		return nil
	}
	var rows []TopicSectionRow
	r.db.Table("topic_section_relation tsr").
		Select("tsr.topic_id, ts.name").
		Joins("JOIN topic_section ts ON ts.id = tsr.topic_section_id").
		Where("tsr.topic_id IN ?", topicIDs).
		Find(&rows)
	return rows
}

// FindTopicIDsWithPoll returns the subset of topicIDs that have at
// least one row in topic_poll.
func (r *SearchRepository) FindTopicIDsWithPoll(topicIDs []int) map[int]bool {
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

// SearchReplies searches topic replies by content. Identity is hydrated by
// the service layer via userclient.
func (r *SearchRepository) SearchReplies(keywords []string, page, limit int) (rows []ReplyRow, total int64) {
	// A reply's text is in r.content (the legacy multi-target rows were folded
	// into it by the Phase-4 migration), so snippet + keyword match use it.
	query := r.db.Table("topic_reply r").
		Select(`r.id, r.topic_id, t.title AS topic_title, t.user_id AS topic_user_id,
			SUBSTRING(COALESCE(r.content, ''), 1, 233) AS content, r.floor,
			r.user_id, r.created`).
		Joins("LEFT JOIN topic t ON t.id = r.topic_id").
		// Exclude moderation-hidden replies (T&S `hide`, migration 055).
		Where("r.status = 0")
	for _, kw := range keywords {
		query = query.Where("r.content ILIKE ?", "%"+kw+"%")
	}

	query.Count(&total)
	query.Order("r.created DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows)
	return
}

// SearchComments searches topic comments by content. Identity is hydrated by
// the service layer via userclient.
func (r *SearchRepository) SearchComments(keywords []string, page, limit int) (rows []CommentRow, total int64) {
	query := r.db.Table("topic_comment c").
		Select(`c.id, c.topic_id, t.title AS topic_title, t.user_id AS topic_user_id,
			SUBSTRING(c.content, 1, 233) AS content,
			c.user_id, c.created`).
		Joins("LEFT JOIN topic t ON t.id = c.topic_id").
		// Exclude moderation-hidden comments (T&S `hide`, migration 055).
		Where("c.status = 0")
	for _, kw := range keywords {
		query = query.Where("c.content ILIKE ?", "%"+kw+"%")
	}

	query.Count(&total)
	query.Order("c.created DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows)
	return
}
