package repository

import (
	userModel "kun-galgame-api/internal/user/model"

	"gorm.io/gorm"
)

type ImageRepository struct {
	db *gorm.DB
}

func NewImageRepository(db *gorm.DB) *ImageRepository {
	return &ImageRepository{db: db}
}

// GetDailyCount returns the user's current daily image upload count from the
// kungal_user_state table (the OAuth-migration successor to user.daily_*).
func (r *ImageRepository) GetDailyCount(userID int) (int, error) {
	var s userModel.KungalUserState
	err := r.db.Select("daily_image_count").
		Where("user_id = ?", userID).First(&s).Error
	return s.DailyImageCount, err
}

// IncrementDailyCount atomically increments the user's daily image upload count.
func (r *ImageRepository) IncrementDailyCount(userID int) {
	r.db.Model(&userModel.KungalUserState{}).Where("user_id = ?", userID).
		Update("daily_image_count", gorm.Expr("daily_image_count + 1"))
}
