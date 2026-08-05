package repository

import (
	"kun-galgame-api/internal/user/model"

	"gorm.io/gorm"
)

// UserStatsRepository owns the per-user aggregate queries that power the
// profile page, the status badge, and the floating hover card stats.
type UserStatsRepository struct {
	db *gorm.DB
}

func NewUserStatsRepository(db *gorm.DB) *UserStatsRepository {
	return &UserStatsRepository{db: db}
}

func (r *UserStatsRepository) DB() *gorm.DB { return r.db }

type UserStats = model.UserStats

// GetUserStats returns contribution + interaction counts for the profile page.
func (r *UserStatsRepository) GetUserStats(userID int) (*model.UserStats, error) {
	var stats model.UserStats
	err := r.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM topic WHERE user_id = @userID) AS topic,
			(SELECT COUNT(*) FROM topic_poll WHERE user_id = @userID) AS topic_poll,
			(SELECT COUNT(*) FROM topic_reply WHERE user_id = @userID AND status = 0) AS reply_created,
			(SELECT COUNT(*) FROM topic_comment WHERE user_id = @userID AND status = 0) AS comment_created,
			-- galgame_comment (now "all community comment areas") is overlaid by the
			-- service from the community primitive's visible_posts (charter step 06a),
			-- not counted from the frozen galgame_comment table here.
			(SELECT COUNT(*) FROM galgame_rating WHERE user_id = @userID) AS galgame_rating,
			(SELECT COUNT(*) FROM galgame_resource WHERE user_id = @userID) AS galgame_resource,
			(SELECT COUNT(*) FROM galgame_website WHERE user_id = @userID) AS galgame_toolset,
			(SELECT COUNT(*) FROM galgame_toolset_resource WHERE user_id = @userID) AS galgame_toolset_resource,
			(SELECT COUNT(*) FROM topic_upvote WHERE topic_id IN (SELECT id FROM topic WHERE user_id = @userID)) AS upvote,
			(SELECT COUNT(*) FROM topic_reaction WHERE reaction = 'like' AND topic_id IN (SELECT id FROM topic WHERE user_id = @userID)) AS "like",
			(SELECT COUNT(*) FROM topic_reaction WHERE reaction = 'dislike' AND topic_id IN (SELECT id FROM topic WHERE user_id = @userID)) AS dislike,
			(SELECT COUNT(*) FROM topic WHERE user_id = @userID AND created >= CURRENT_DATE) AS daily_topic_count
	`, map[string]any{"userID": userID}).Scan(&stats).Error
	return &stats, err
}

// CountUnreadMessages counts 1:1 messages addressed to the user that are
// still in the `unread` state. mutedLocal (from the user's notification
// preferences — migration 053) excludes those message.type values from the
// count so the red dot doesn't light for categories the user muted; the rows
// themselves are untouched and still visible in the notification center.
func (r *UserStatsRepository) CountUnreadMessages(userID int, mutedLocal []string) (int64, error) {
	var count int64
	q := r.db.Table("message").
		Where("receiver_id = ? AND status = 'unread'", userID)
	if len(mutedLocal) > 0 {
		q = q.Not("type IN ?", mutedLocal)
	}
	err := q.Count(&count).Error
	return count, err
}

// CountUnreadSystemMessages counts admin broadcasts the user hasn't read
// yet (id > user's HWM cursor). A missing cursor row is treated as 0 so
// fresh users see the full backlog as unread.
func (r *UserStatsRepository) CountUnreadSystemMessages(userID int) (int64, error) {
	var count int64
	err := r.db.Table("system_message").
		Where(`id > COALESCE(
			(SELECT last_read_message_id
			 FROM system_message_read_state
			 WHERE user_id = ?), 0)`, userID).
		Count(&count).Error
	return count, err
}

// CountUnreadChatMessages counts chat messages in the user's rooms that
// haven't been read by them (excluding their own messages).
func (r *UserStatsRepository) CountUnreadChatMessages(userID int) (int64, error) {
	var count int64
	err := r.db.Table("chat_message").
		Where("sender_id != ?", userID).
		Where("chat_room_id IN (SELECT chat_room_id FROM chat_room_participant WHERE user_id = ?)", userID).
		Where("id NOT IN (SELECT chat_message_id FROM chat_message_read_by WHERE user_id = ?)", userID).
		Count(&count).Error
	return count, err
}

// ──────────────────────────────────────────
// Floating hover card aggregates
// ──────────────────────────────────────────

// FloatingStatsRow aggregates contribution counts for the hover card.
type FloatingStatsRow struct {
	TopicCount        int64 `gorm:"column:topic_count"`
	TopicReplyCount   int64 `gorm:"column:topic_reply_count"`
	TopicCommentCount int64 `gorm:"column:topic_comment_count"`
	ResourceCount     int64 `gorm:"column:resource_count"`
}

// FindFloatingStats runs the four aggregate sub-queries powering the hover
// card (topic / reply / comment union / resource counts).
func (r *UserStatsRepository) FindFloatingStats(userID int) FloatingStatsRow {
	var stats FloatingStatsRow
	r.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM topic WHERE user_id = @userID) AS topic_count,
			(SELECT COUNT(*) FROM topic_reply WHERE user_id = @userID AND status = 0) AS topic_reply_count,
			-- Only the local topic_comment count here; the community comment areas
			-- (galgame + website + …) are added by the service from the primitive's
			-- visible_posts (charter step 06a), replacing the old galgame_comment +
			-- galgame_website_comment frozen-table terms.
			(SELECT COUNT(*) FROM topic_comment WHERE user_id = @userID AND status = 0) AS topic_comment_count,
			(SELECT COUNT(*) FROM galgame_resource WHERE user_id = @userID) AS resource_count
	`, map[string]any{"userID": userID}).Scan(&stats)
	return stats
}
