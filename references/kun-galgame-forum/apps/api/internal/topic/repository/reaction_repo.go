package repository

import (
	"time"

	"kun-galgame-api/internal/topic/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Reaction data access for topics + replies (unified 'like'/'dislike'/emoji).
// Write methods take the caller's tx so toggles stay atomic with count + side
// effects. Read/aggregate methods land in a later step.

// ──────────────────────────────────────────
// Topic reactions
// ──────────────────────────────────────────

func (r *TopicRepository) HasReaction(tx *gorm.DB, topicID, userID int, reaction string) (bool, error) {
	var count int64
	err := tx.Model(&model.TopicReaction{}).
		Where("topic_id = ? AND user_id = ? AND reaction = ?", topicID, userID, reaction).
		Count(&count).Error
	return count > 0, err
}

func (r *TopicRepository) AddReaction(tx *gorm.DB, topicID, userID int, reaction string) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.TopicReaction{TopicID: topicID, UserID: userID, Reaction: reaction}).Error
}

func (r *TopicRepository) RemoveReaction(tx *gorm.DB, topicID, userID int, reaction string) error {
	return tx.Where("topic_id = ? AND user_id = ? AND reaction = ?", topicID, userID, reaction).
		Delete(&model.TopicReaction{}).Error
}

// ──────────────────────────────────────────
// Reply reactions
// ──────────────────────────────────────────

func (r *ReplyRepository) HasReplyReaction(tx *gorm.DB, replyID, userID int, reaction string) (bool, error) {
	var count int64
	err := tx.Model(&model.TopicReplyReaction{}).
		Where("topic_reply_id = ? AND user_id = ? AND reaction = ?", replyID, userID, reaction).
		Count(&count).Error
	return count > 0, err
}

func (r *ReplyRepository) AddReplyReaction(tx *gorm.DB, replyID, userID int, reaction string) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.TopicReplyReaction{TopicReplyID: replyID, UserID: userID, Reaction: reaction}).Error
}

func (r *ReplyRepository) RemoveReplyReaction(tx *gorm.DB, replyID, userID int, reaction string) error {
	return tx.Where("topic_reply_id = ? AND user_id = ? AND reaction = ?", replyID, userID, reaction).
		Delete(&model.TopicReplyReaction{}).Error
}

// ──────────────────────────────────────────
// Reaction aggregates (read / display)
// ──────────────────────────────────────────

// reactionAvatarCap bounds reactor ids fetched (and shown as avatars) per
// reaction; the remainder collapse to a "+N" overflow badge on the FE.
const reactionAvatarCap = 3

// ReactionRow is a windowed reactor row: the reaction key, one reactor, and the
// reaction's total count (repeated across the group's rows).
type ReactionRow struct {
	Reaction string `gorm:"column:reaction"`
	UserID   int    `gorm:"column:user_id"`
	Cnt      int    `gorm:"column:cnt"`
}

// GetTopicReactions returns up to reactionAvatarCap reactor ids per reaction for
// a topic, each row carrying that reaction's total count.
func (r *TopicRepository) GetTopicReactions(topicID int) ([]ReactionRow, error) {
	var rows []ReactionRow
	err := r.db.Raw(`
		SELECT reaction, user_id, cnt FROM (
			SELECT reaction, user_id,
				row_number() OVER (PARTITION BY reaction ORDER BY created) AS rn,
				count(*) OVER (PARTITION BY reaction) AS cnt
			FROM topic_reaction WHERE topic_id = ?
		) t WHERE rn <= ? ORDER BY reaction, rn`, topicID, reactionAvatarCap).Scan(&rows).Error
	return rows, err
}

// GetUserTopicReactions returns the reaction keys the user holds on a topic.
func (r *TopicRepository) GetUserTopicReactions(topicID, userID int) ([]string, error) {
	if userID <= 0 {
		return nil, nil
	}
	var keys []string
	err := r.db.Model(&model.TopicReaction{}).
		Where("topic_id = ? AND user_id = ?", topicID, userID).Pluck("reaction", &keys).Error
	return keys, err
}

// ReactionHistoryRow is one reaction event: who reacted, with what key, and when.
type ReactionHistoryRow struct {
	UserID   int       `gorm:"column:user_id"`
	Reaction string    `gorm:"column:reaction"`
	Created  time.Time `gorm:"column:created"`
}

// GetTopicReactionHistory returns a topic's reaction events newest-first, capped,
// for the 查看历史 list — one row per reaction event.
func (r *TopicRepository) GetTopicReactionHistory(topicID, limit int) ([]ReactionHistoryRow, error) {
	var rows []ReactionHistoryRow
	err := r.db.Model(&model.TopicReaction{}).
		Select("user_id, reaction, created").
		Where("topic_id = ?", topicID).
		Order("created DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// ReplyReactionRow is GetRepliesReactions' windowed row (adds the reply id).
type ReplyReactionRow struct {
	TopicReplyID int    `gorm:"column:topic_reply_id"`
	Reaction     string `gorm:"column:reaction"`
	UserID       int    `gorm:"column:user_id"`
	Cnt          int    `gorm:"column:cnt"`
}

// GetRepliesReactions batches GetTopicReactions across a list of replies.
func (r *ReplyRepository) GetRepliesReactions(replyIDs []int) ([]ReplyReactionRow, error) {
	out := []ReplyReactionRow{}
	if len(replyIDs) == 0 {
		return out, nil
	}
	err := r.db.Raw(`
		SELECT topic_reply_id, reaction, user_id, cnt FROM (
			SELECT topic_reply_id, reaction, user_id,
				row_number() OVER (PARTITION BY topic_reply_id, reaction ORDER BY created) AS rn,
				count(*) OVER (PARTITION BY topic_reply_id, reaction) AS cnt
			FROM topic_reply_reaction WHERE topic_reply_id IN ?
		) t WHERE rn <= ? ORDER BY topic_reply_id, reaction, rn`, replyIDs, reactionAvatarCap).Scan(&out).Error
	return out, err
}

// GetUserRepliesReactions returns the reaction keys the user holds, per reply.
func (r *ReplyRepository) GetUserRepliesReactions(replyIDs []int, userID int) (map[int][]string, error) {
	out := map[int][]string{}
	if userID <= 0 || len(replyIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		TopicReplyID int    `gorm:"column:topic_reply_id"`
		Reaction     string `gorm:"column:reaction"`
	}
	if err := r.db.Table("topic_reply_reaction").
		Select("topic_reply_id, reaction").
		Where("topic_reply_id IN ? AND user_id = ?", replyIDs, userID).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicReplyID] = append(out[row.TopicReplyID], row.Reaction)
	}
	return out, nil
}

// findReactionStatus resolves which of `ids` the user reacted to with `reaction`
// in `table` (the reaction-aware sibling of findInteractionStatus; shared by the
// topic + reply like/dislike status reads now that those live in the reaction
// tables).
func findReactionStatus(db *gorm.DB, table, fkCol, reaction string, userID int, ids []int) (map[int]bool, error) {
	out := make(map[int]bool)
	if len(ids) == 0 || userID == 0 {
		return out, nil
	}
	var foundIDs []int
	if err := db.Table(table).
		Where("user_id = ? AND reaction = ? AND "+fkCol+" IN ?", userID, reaction, ids).
		Pluck(fkCol, &foundIDs).Error; err != nil {
		return out, err
	}
	for _, id := range foundIDs {
		out[id] = true
	}
	return out, nil
}
