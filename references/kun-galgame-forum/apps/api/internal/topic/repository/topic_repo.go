package repository

import (
	"time"

	"kun-galgame-api/internal/infrastructure/viewstats"
	"kun-galgame-api/internal/topic/model"

	"gorm.io/gorm"
)

// TopicRepository owns the core topic lifecycle: single-row CRUD, daily-limit
// count, interaction-check predicates, interaction row toggles + counter
// adjustments, and the topic-author projection.
//
// Sibling repos in this package own the list queries and the tag/section
// relations:
//   - TopicListRepository     (list_repo.go)
//   - TopicTaxonomyRepository (taxonomy_repo.go)
type TopicRepository struct {
	db *gorm.DB
}

func NewTopicRepository(db *gorm.DB) *TopicRepository {
	return &TopicRepository{db: db}
}

func (r *TopicRepository) DB() *gorm.DB {
	return r.db
}

// ──────────────────────────────────────────
// Core CRUD
// ──────────────────────────────────────────

func (r *TopicRepository) FindByID(id int) (*model.Topic, error) {
	var topic model.Topic
	err := r.db.First(&topic, id).Error
	return &topic, err
}

// FindReplyByID looks up a single reply row — exposed here (rather than
// via a separate ReplyRepository injection) so the detail service can
// hydrate topic.best_answer_id → reply summary without growing the
// TopicService constructor's dependency list.
func (r *TopicRepository) FindReplyByID(id int) (*model.TopicReply, error) {
	var reply model.TopicReply
	err := r.db.First(&reply, id).Error
	return &reply, err
}

func (r *TopicRepository) Create(topic *model.Topic) error {
	return r.db.Create(topic).Error
}

func (r *TopicRepository) UpdateFields(id int, fields map[string]any) error {
	return r.db.Model(&model.Topic{}).Where("id = ?", id).Updates(fields).Error
}

func (r *TopicRepository) IncrementView(id int) error {
	err := r.db.Model(&model.Topic{}).Where("id = ?", id).
		Update("view", gorm.Expr("view + 1")).Error
	_ = viewstats.BumpDaily(r.db, viewstats.TopicDaily, id)
	return err
}

// ──────────────────────────────────────────
// Interaction checks
// ──────────────────────────────────────────

func (r *TopicRepository) HasUserLiked(userID, topicID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.TopicReaction{}).
		Where("user_id = ? AND topic_id = ? AND reaction = 'like'", userID, topicID).Count(&count).Error
	return count > 0, err
}

func (r *TopicRepository) HasUserDisliked(userID, topicID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.TopicReaction{}).
		Where("user_id = ? AND topic_id = ? AND reaction = 'dislike'", userID, topicID).Count(&count).Error
	return count > 0, err
}

func (r *TopicRepository) HasUserFavorited(userID, topicID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.TopicFavorite{}).Where("user_id = ? AND topic_id = ?", userID, topicID).Count(&count).Error
	return count > 0, err
}

func (r *TopicRepository) HasUserUpvoted(userID, topicID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.TopicUpvote{}).Where("user_id = ? AND topic_id = ?", userID, topicID).Count(&count).Error
	return count > 0, err
}

// HasUserUpvotedTx is the in-transaction guard for Upvote: reads within the
// caller's tx so the once-per-user check is consistent with the FOR UPDATE lock
// on the user-state row (which serializes concurrent upvotes by the same user).
func (r *TopicRepository) HasUserUpvotedTx(tx *gorm.DB, userID, topicID int) (bool, error) {
	var count int64
	err := tx.Model(&model.TopicUpvote{}).Where("user_id = ? AND topic_id = ?", userID, topicID).Count(&count).Error
	return count > 0, err
}

// ──────────────────────────────────────────
// Daily limit
// ──────────────────────────────────────────

func (r *TopicRepository) CountTodayTopicsByUser(tx *gorm.DB, userID int) (int64, error) {
	var count int64
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	err := tx.Model(&model.Topic{}).
		Where("user_id = ? AND created >= ?", userID, oneDayAgo).
		Count(&count).Error
	return count, err
}

// ──────────────────────────────────────────
// Poll existence check
// ──────────────────────────────────────────

func (r *TopicRepository) HasPoll(topicID int) (bool, error) {
	var count int64
	err := r.db.Model(&model.TopicPoll{}).Where("topic_id = ?", topicID).Count(&count).Error
	return count > 0, err
}

// FindTopicIDsWithPoll returns a set of the topic IDs (from the given
// list) that have at least one row in topic_poll. Used by the list
// mappers so the FE can render the "投票" badge on cards — was
// previously hardcoded `false`, leaving the badge permanently absent
// from topic / resource list pages.
func (r *TopicRepository) FindTopicIDsWithPoll(topicIDs []int) map[int]bool {
	out := map[int]bool{}
	if len(topicIDs) == 0 {
		return out
	}
	var rows []struct {
		TopicID int `gorm:"column:topic_id"`
	}
	r.db.Model(&model.TopicPoll{}).
		Where("topic_id IN ?", topicIDs).
		Select("topic_id").
		Scan(&rows)
	for _, row := range rows {
		out[row.TopicID] = true
	}
	return out
}

// ──────────────────────────────────────────
// Tx-aware write operations (for the service coordinator)
// ──────────────────────────────────────────

// FindByIDTx loads a topic inside a transaction.
func (r *TopicRepository) FindByIDTx(tx *gorm.DB, topicID int) (*model.Topic, error) {
	var topic model.Topic
	err := tx.First(&topic, topicID).Error
	return &topic, err
}

// CreateTopic inserts a Topic row inside the caller tx.
func (r *TopicRepository) CreateTopic(tx *gorm.DB, topic *model.Topic) error {
	return tx.Create(topic).Error
}

// UpdateTopicFields updates arbitrary Topic columns inside the caller tx.
func (r *TopicRepository) UpdateTopicFields(tx *gorm.DB, topicID int, fields map[string]any) error {
	return tx.Model(&model.Topic{}).Where("id = ?", topicID).Updates(fields).Error
}

// TouchStatusUpdateTime bumps status_update_time for a topic — but only while
// the topic is still inside its bump window (see model.BumpCutoff). The
// `created > cutoff` guard is in SQL so an aged-out topic is simply not matched
// (necro-bump prevention: interactions no longer resurface old topics).
func (r *TopicRepository) TouchStatusUpdateTime(tx *gorm.DB, topicID int, t time.Time) error {
	return tx.Model(&model.Topic{}).
		Where("id = ? AND created > ?", topicID, model.BumpCutoff(t)).
		Updates(map[string]any{"status_update_time": t}).Error
}

// ──────────────────────────────────────────
// Interaction rows (tx-aware)
// ──────────────────────────────────────────

// FindTopicLike returns an existing TopicLike row (or gorm.ErrRecordNotFound).
func (r *TopicRepository) FindTopicLike(tx *gorm.DB, userID, topicID int) (*model.TopicLike, error) {
	var existing model.TopicLike
	err := tx.Where("user_id = ? AND topic_id = ?", userID, topicID).First(&existing).Error
	return &existing, err
}

// CreateTopicLike inserts a TopicLike row.
func (r *TopicRepository) CreateTopicLike(tx *gorm.DB, userID, topicID int) error {
	return tx.Create(&model.TopicLike{UserID: userID, TopicID: topicID}).Error
}

// DeleteTopicLike removes a previously fetched TopicLike row.
func (r *TopicRepository) DeleteTopicLike(tx *gorm.DB, like *model.TopicLike) error {
	return tx.Delete(like).Error
}

// FindTopicDislike returns an existing TopicDislike row.
func (r *TopicRepository) FindTopicDislike(tx *gorm.DB, userID, topicID int) (*model.TopicDislike, error) {
	var existing model.TopicDislike
	err := tx.Where("user_id = ? AND topic_id = ?", userID, topicID).First(&existing).Error
	return &existing, err
}

// CreateTopicDislike inserts a TopicDislike row.
func (r *TopicRepository) CreateTopicDislike(tx *gorm.DB, userID, topicID int) error {
	return tx.Create(&model.TopicDislike{UserID: userID, TopicID: topicID}).Error
}

// DeleteTopicDislike removes a previously fetched TopicDislike row.
func (r *TopicRepository) DeleteTopicDislike(tx *gorm.DB, dislike *model.TopicDislike) error {
	return tx.Delete(dislike).Error
}

// FindTopicFavorite returns an existing TopicFavorite row.
func (r *TopicRepository) FindTopicFavorite(tx *gorm.DB, userID, topicID int) (*model.TopicFavorite, error) {
	var existing model.TopicFavorite
	err := tx.Where("user_id = ? AND topic_id = ?", userID, topicID).First(&existing).Error
	return &existing, err
}

// CreateTopicFavorite inserts a TopicFavorite row.
func (r *TopicRepository) CreateTopicFavorite(tx *gorm.DB, userID, topicID int) error {
	return tx.Create(&model.TopicFavorite{UserID: userID, TopicID: topicID}).Error
}

// DeleteTopicFavorite removes a previously fetched TopicFavorite row.
func (r *TopicRepository) DeleteTopicFavorite(tx *gorm.DB, fav *model.TopicFavorite) error {
	return tx.Delete(fav).Error
}

// UserTopicInteractions returns the user's favorited topic ids + the reaction
// keys they hold per topic — feed-card hydration (the shared feed cache can't
// carry per-viewer state). Two cheap indexed reads.
func (r *TopicRepository) UserTopicInteractions(userID int) ([]int, map[int][]string, error) {
	favorited := []int{}
	if err := r.db.Model(&model.TopicFavorite{}).
		Where("user_id = ?", userID).Pluck("topic_id", &favorited).Error; err != nil {
		return nil, nil, err
	}
	var rows []struct {
		TopicID  int    `gorm:"column:topic_id"`
		Reaction string `gorm:"column:reaction"`
	}
	if err := r.db.Table("topic_reaction").
		Select("topic_id, reaction").
		Where("user_id = ?", userID).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	reactions := map[int][]string{}
	for _, row := range rows {
		reactions[row.TopicID] = append(reactions[row.TopicID], row.Reaction)
	}
	return favorited, reactions, nil
}

// CreateTopicUpvote inserts a TopicUpvote row (duplicates allowed). description
// is the optional "why I pushed it" one-liner (” when omitted).
func (r *TopicRepository) CreateTopicUpvote(tx *gorm.DB, userID, topicID int, description string) error {
	return tx.Create(&model.TopicUpvote{UserID: userID, TopicID: topicID, Description: description}).Error
}

// TopicUpvoteRow is one 推话题 record: who pushed, their one-liner, and when.
type TopicUpvoteRow struct {
	ID          int       `gorm:"column:id"`
	UserID      int       `gorm:"column:user_id"`
	Description string    `gorm:"column:description"`
	Created     time.Time `gorm:"column:created"`
}

// FetchTopicUpvotes returns a topic's push records, newest first, capped at limit.
func (r *TopicRepository) FetchTopicUpvotes(topicID, limit int) ([]TopicUpvoteRow, error) {
	var rows []TopicUpvoteRow
	err := r.db.Table("topic_upvote").
		Select("id, user_id, description, created").
		Where("topic_id = ?", topicID).
		Order("created DESC, id DESC").
		Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// AdjustLikeCount adjusts topic.like_count by delta.
func (r *TopicRepository) AdjustLikeCount(tx *gorm.DB, topicID, delta int) error {
	return tx.Model(&model.Topic{}).Where("id = ?", topicID).
		Update("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// AdjustDislikeCount adjusts topic.dislike_count by delta.
func (r *TopicRepository) AdjustDislikeCount(tx *gorm.DB, topicID, delta int) error {
	return tx.Model(&model.Topic{}).Where("id = ?", topicID).
		Update("dislike_count", gorm.Expr("dislike_count + ?", delta)).Error
}

// AdjustFavoriteCount adjusts topic.favorite_count by delta.
func (r *TopicRepository) AdjustFavoriteCount(tx *gorm.DB, topicID, delta int) error {
	return tx.Model(&model.Topic{}).Where("id = ?", topicID).
		Update("favorite_count", gorm.Expr("favorite_count + ?", delta)).Error
}

// ApplyUpvoteCountAndTime bumps upvote_count and sets upvote_time /
// status_update_time. upvote_count and upvote_time always update; only the
// status_update_time bump is gated on the bump window (necro-bump prevention),
// via an in-SQL CASE so the whole thing stays one race-free statement.
func (r *TopicRepository) ApplyUpvoteCountAndTime(tx *gorm.DB, topicID int, t time.Time) error {
	return tx.Model(&model.Topic{}).Where("id = ?", topicID).Updates(map[string]any{
		"upvote_count": gorm.Expr("upvote_count + 1"),
		"upvote_time":  &t,
		"status_update_time": gorm.Expr(
			"CASE WHEN created > ? THEN ? ELSE status_update_time END", model.BumpCutoff(t), t),
	}).Error
}

// ──────────────────────────────────────────
// Topic author projection (identity hydrated via pkg/userclient by the
// service layer; moemoepoint comes from kungal_user_state).
// ──────────────────────────────────────────

// TopicAuthorUser is the minimal user projection needed on the topic detail page.
type TopicAuthorUser struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar"`
	Moemoepoint int    `json:"moemoepoint"`
}
