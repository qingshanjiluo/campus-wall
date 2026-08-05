package repository

import (
	"kun-galgame-api/internal/galgame/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommunityPostRepository owns the LOCAL bookkeeping for community-backed galgame
// comments: the galgame_post_like table (display like count + "我赞过") and the
// read side of the legacy-id → community map (deep-link continuity). The
// community primitive owns the post content itself over S2S; this repo never
// writes it. All methods target tables created by migration 057.
type CommunityPostRepository struct {
	db *gorm.DB
}

func NewCommunityPostRepository(db *gorm.DB) *CommunityPostRepository {
	return &CommunityPostRepository{db: db}
}

func (r *CommunityPostRepository) DB() *gorm.DB { return r.db }

// EnsureLike inserts a like row (idempotent — ON CONFLICT DO NOTHING on the
// unique (post_id, user_id)). Driven by the community toggle's added=true.
func (r *CommunityPostRepository) EnsureLike(postID int64, userID int) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.GalgamePostLike{PostID: postID, UserID: userID}).Error
}

// RemoveLike deletes a like row (idempotent — a missing row is a no-op). Driven
// by the community toggle's added=false.
func (r *CommunityPostRepository) RemoveLike(postID int64, userID int) error {
	return r.db.Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.GalgamePostLike{}).Error
}

// CountLikes returns like counts for the given post ids in one grouped query
// (0 for a post with no likes — absent from the map). Empty input → empty map.
func (r *CommunityPostRepository) CountLikes(postIDs []int64) map[int64]int {
	out := make(map[int64]int, len(postIDs))
	if len(postIDs) == 0 {
		return out
	}
	var rows []struct {
		PostID int64 `gorm:"column:post_id"`
		N      int   `gorm:"column:n"`
	}
	r.db.Table("galgame_post_like").
		Select("post_id, COUNT(*) AS n").
		Where("post_id IN ?", postIDs).
		Group("post_id").
		Scan(&rows)
	for _, row := range rows {
		out[row.PostID] = row.N
	}
	return out
}

// LikedSet returns the subset of postIDs the viewer has liked. Anonymous
// (userID<=0) or empty input → empty set without a DB hit.
func (r *CommunityPostRepository) LikedSet(userID int, postIDs []int64) map[int64]bool {
	out := make(map[int64]bool, len(postIDs))
	if userID <= 0 || len(postIDs) == 0 {
		return out
	}
	var rows []struct {
		PostID int64 `gorm:"column:post_id"`
	}
	r.db.Table("galgame_post_like").
		Where("user_id = ? AND post_id IN ?", userID, postIDs).
		Select("post_id").
		Scan(&rows)
	for _, row := range rows {
		out[row.PostID] = true
	}
	return out
}

// FindMapByLegacyID resolves an OLD galgame_comment id to its migrated community
// (thread, post). Returns nil (no error) when the id was never migrated, so the
// locate endpoint can answer 404.
func (r *CommunityPostRepository) FindMapByLegacyID(legacyID int) (*model.GalgameCommentCommunityMap, error) {
	var row model.GalgameCommentCommunityMap
	err := r.db.Where("old_comment_id = ?", legacyID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
