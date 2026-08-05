package repository

import (
	"time"

	"kun-galgame-api/internal/topic/model"

	"gorm.io/gorm"
)

type TopicDraftRepository struct {
	db *gorm.DB
}

func NewTopicDraftRepository(db *gorm.DB) *TopicDraftRepository {
	return &TopicDraftRepository{db: db}
}

// DraftListRow is the metadata-only projection for the list view (content is
// fetched only on load); summary is a short raw-markdown prefix for display.
type DraftListRow struct {
	ID      int
	Title   string
	Summary string
	Updated time.Time
}

func (r *TopicDraftRepository) CountByUser(userID int) (int64, error) {
	var n int64
	err := r.db.Model(&model.TopicDraft{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

func (r *TopicDraftRepository) Create(d *model.TopicDraft) error {
	return r.db.Create(d).Error
}

func (r *TopicDraftRepository) ListByUser(userID int) ([]DraftListRow, error) {
	var rows []DraftListRow
	err := r.db.Model(&model.TopicDraft{}).
		Select("id, title, LEFT(content, 120) AS summary, updated").
		Where("user_id = ?", userID).
		Order("updated DESC").
		Scan(&rows).Error
	return rows, err
}

func (r *TopicDraftRepository) GetByIDForUser(id, userID int) (*model.TopicDraft, error) {
	var d model.TopicDraft
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *TopicDraftRepository) DeleteForUser(id, userID int) (int64, error) {
	res := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.TopicDraft{})
	return res.RowsAffected, res.Error
}
