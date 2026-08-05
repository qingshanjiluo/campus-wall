package repository

import (
	"kun-galgame-api/internal/galgame/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GalgameInteractionRepository owns the user→galgame interaction rows
// (likes, favorites) and the counter side-effects on galgame_local.
type GalgameInteractionRepository struct {
	db *gorm.DB
}

func NewGalgameInteractionRepository(db *gorm.DB) *GalgameInteractionRepository {
	return &GalgameInteractionRepository{db: db}
}

func (r *GalgameInteractionRepository) DB() *gorm.DB { return r.db }

// UserInteraction reports whether the user has liked / favorited a galgame.
// "favorited" now means the game is in >=1 of the user's collections (收藏夹);
// the denormalized galgame_collection_item.user_id keeps this an index-only read.
func (r *GalgameInteractionRepository) UserInteraction(userID, galgameID int) (liked, favorited bool) {
	if userID <= 0 {
		return false, false
	}
	var lc, fc int64
	r.db.Model(&model.GalgameLike{}).
		Where("user_id = ? AND galgame_id = ?", userID, galgameID).Count(&lc)
	r.db.Model(&model.GalgameCollectionItem{}).
		Where("user_id = ? AND galgame_id = ?", userID, galgameID).Count(&fc)
	return lc > 0, fc > 0
}

// UserGalgameInteractions returns every galgame id the user has liked / favorited.
// Hydrates feed-card like/favorite state (the shared feed cache can't carry
// per-user state). "favorited" is the DISTINCT set of galgames the user has in
// any collection. Anonymous user → empty (non-nil) slices.
func (r *GalgameInteractionRepository) UserGalgameInteractions(userID int) (liked, favorited []int) {
	liked = []int{}
	favorited = []int{}
	if userID <= 0 {
		return
	}
	r.db.Model(&model.GalgameLike{}).
		Where("user_id = ?", userID).Pluck("galgame_id", &liked)
	r.db.Model(&model.GalgameCollectionItem{}).
		Distinct("galgame_id").
		Where("user_id = ?", userID).Pluck("galgame_id", &favorited)
	return
}

// ToggleLike inserts/removes a like row atomically within a caller-supplied tx.
// Returns whether the result is "now liked".
//
// Lazy-creates the local stub before incrementing: pending submissions don't
// get a stub at submit time, so the first interaction (which may be the
// submitter liking their own pending row) must INSERT one or the UPDATE
// would silently affect 0 rows.
func (r *GalgameInteractionRepository) ToggleLike(tx *gorm.DB, userID, galgameID int) (liked bool) {
	var existing model.GalgameLike
	result := tx.Where("user_id = ? AND galgame_id = ?", userID, galgameID).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&model.GalgameLocal{ID: galgameID})
		tx.Create(&model.GalgameLike{UserID: userID, GalgameID: galgameID})
		tx.Model(&model.GalgameLocal{}).Where("id = ?", galgameID).
			Update("like_count", gorm.Expr("like_count + 1"))
		return true
	}

	tx.Delete(&existing)
	tx.Model(&model.GalgameLocal{}).Where("id = ?", galgameID).
		Update("like_count", gorm.Expr("like_count - 1"))
	return false
}

// Favorite is no longer a per-galgame toggle row here — it moved to collections
// (galgame_collection_item). GalgameCollectionRepository.AddItem/RemoveItem +
// AdjustGalgameFavoriteCount own the write path and the favorite_count counter.
