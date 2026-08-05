package repository

import (
	"kun-galgame-api/internal/galgame/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GalgameCollectionRepository owns the galgame collection tables (收藏夹):
// galgame_collection, galgame_collection_item, galgame_collection_viewer, plus
// the favorite_count side-effect on galgame_local. Concrete struct, ownership is
// enforced in the WHERE clause (mirrors TopicDraftRepository).
type GalgameCollectionRepository struct {
	db *gorm.DB
}

func NewGalgameCollectionRepository(db *gorm.DB) *GalgameCollectionRepository {
	return &GalgameCollectionRepository{db: db}
}

func (r *GalgameCollectionRepository) DB() *gorm.DB { return r.db }

// ── Collection CRUD ──

func (r *GalgameCollectionRepository) Create(tx *gorm.DB, c *model.GalgameCollection) error {
	return tx.Create(c).Error
}

func (r *GalgameCollectionRepository) Save(tx *gorm.DB, c *model.GalgameCollection) error {
	return tx.Save(c).Error
}

func (r *GalgameCollectionRepository) CountByUser(userID int) (int64, error) {
	var n int64
	err := r.db.Model(&model.GalgameCollection{}).Where("user_id = ?", userID).Count(&n).Error
	return n, err
}

// GetByID loads a collection without an owner scope (for the access-checked
// detail read of someone else's public/restricted collection).
func (r *GalgameCollectionRepository) GetByID(id int) (*model.GalgameCollection, error) {
	var c model.GalgameCollection
	if err := r.db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// GetByIDForUser loads a collection only if it belongs to userID (owner-scoped
// mutations: 404 for missing OR not-yours).
func (r *GalgameCollectionRepository) GetByIDForUser(id, userID int) (*model.GalgameCollection, error) {
	var c model.GalgameCollection
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListAllByUser returns all of a user's collections, default first then newest,
// for the picker modal.
func (r *GalgameCollectionRepository) ListAllByUser(userID int) ([]model.GalgameCollection, error) {
	var rows []model.GalgameCollection
	err := r.db.Where("user_id = ?", userID).
		Order("is_default DESC, updated DESC").
		Find(&rows).Error
	return rows, err
}

// ListForOwnerVisible returns a page of ownerID's collections that viewerID may
// see: the owner sees all; others see public + restricted-they're-listed-on.
func (r *GalgameCollectionRepository) ListForOwnerVisible(ownerID, viewerID, page, limit int) ([]model.GalgameCollection, int64, error) {
	q := r.db.Model(&model.GalgameCollection{}).Where("user_id = ?", ownerID)
	if viewerID != ownerID {
		// Explicit outer parens so the OR can never escape the AND user_id scope
		// (a raw-string Where is not auto-parenthesized), which would otherwise
		// leak other owners' restricted collections.
		q = q.Where(
			"(visibility = ? OR (visibility = ? AND id IN (?)))",
			model.CollectionPublic,
			model.CollectionRestricted,
			r.db.Model(&model.GalgameCollectionViewer{}).
				Select("collection_id").Where("user_id = ?", viewerID),
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.GalgameCollection
	err := q.Order("is_default DESC, updated DESC").
		Offset((page - 1) * limit).Limit(limit).
		Find(&rows).Error
	return rows, total, err
}

// DeleteForUser removes a collection (items + viewers cascade via FK).
func (r *GalgameCollectionRepository) DeleteForUser(tx *gorm.DB, id, userID int) (int64, error) {
	res := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.GalgameCollection{})
	return res.RowsAffected, res.Error
}

// ── Viewers (restricted allow-list) ──

func (r *GalgameCollectionRepository) ListViewerIDs(collectionID int) ([]int, error) {
	ids := []int{}
	err := r.db.Model(&model.GalgameCollectionViewer{}).
		Where("collection_id = ?", collectionID).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *GalgameCollectionRepository) IsViewer(collectionID, userID int) bool {
	var n int64
	r.db.Model(&model.GalgameCollectionViewer{}).
		Where("collection_id = ? AND user_id = ?", collectionID, userID).Count(&n)
	return n > 0
}

// ReplaceViewers swaps the full viewer allow-list for a collection.
func (r *GalgameCollectionRepository) ReplaceViewers(tx *gorm.DB, collectionID int, viewerIDs []int) error {
	if err := tx.Where("collection_id = ?", collectionID).
		Delete(&model.GalgameCollectionViewer{}).Error; err != nil {
		return err
	}
	if len(viewerIDs) == 0 {
		return nil
	}
	rows := make([]model.GalgameCollectionViewer, 0, len(viewerIDs))
	for _, uid := range viewerIDs {
		rows = append(rows, model.GalgameCollectionViewer{CollectionID: collectionID, UserID: uid})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

// ── Items / membership ──

// OwnsAll reports whether every collection id belongs to userID.
func (r *GalgameCollectionRepository) OwnsAll(userID int, ids []int) bool {
	if len(ids) == 0 {
		return true
	}
	var n int64
	r.db.Model(&model.GalgameCollection{}).
		Where("user_id = ? AND id IN ?", userID, ids).Count(&n)
	return int(n) == len(uniqueInts(ids))
}

// UserCollectionIDsForGalgame returns the ids of the user's collections that
// currently contain the galgame.
func (r *GalgameCollectionRepository) UserCollectionIDsForGalgame(tx *gorm.DB, userID, galgameID int) ([]int, error) {
	ids := []int{}
	err := tx.Model(&model.GalgameCollectionItem{}).
		Where("user_id = ? AND galgame_id = ?", userID, galgameID).
		Pluck("collection_id", &ids).Error
	return ids, err
}

// ListItemGalgameIDs returns a page of a collection's galgame ids, newest-added first.
func (r *GalgameCollectionRepository) ListItemGalgameIDs(collectionID, page, limit int) ([]int, int64, error) {
	var total int64
	if err := r.db.Model(&model.GalgameCollectionItem{}).
		Where("collection_id = ?", collectionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	ids := []int{}
	err := r.db.Model(&model.GalgameCollectionItem{}).
		Where("collection_id = ?", collectionID).
		Order("created DESC").
		Offset((page-1)*limit).Limit(limit).
		Pluck("galgame_id", &ids).Error
	return ids, total, err
}

// previewRow is the (collection_id, galgame_id) projection for cover collages.
type previewRow struct {
	CollectionID int
	GalgameID    int
}

// PreviewGalgameIDs returns up to `perCollection` newest galgame ids per
// collection, for cover collages on the grid. One query for the whole page.
func (r *GalgameCollectionRepository) PreviewGalgameIDs(collectionIDs []int, perCollection int) (map[int][]int, error) {
	out := map[int][]int{}
	if len(collectionIDs) == 0 {
		return out, nil
	}
	var rows []previewRow
	if err := r.db.Model(&model.GalgameCollectionItem{}).
		Select("collection_id, galgame_id").
		Where("collection_id IN ?", collectionIDs).
		Order("collection_id, created DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if len(out[row.CollectionID]) < perCollection {
			out[row.CollectionID] = append(out[row.CollectionID], row.GalgameID)
		}
	}
	return out, nil
}

// AddItem inserts a membership row and bumps item_count + the collection's
// updated stamp. Idempotent (ON CONFLICT DO NOTHING) so a double-add is a no-op.
func (r *GalgameCollectionRepository) AddItem(tx *gorm.DB, collectionID, galgameID, userID int) error {
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.GalgameCollectionItem{CollectionID: collectionID, GalgameID: galgameID, UserID: userID})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	return tx.Model(&model.GalgameCollection{}).Where("id = ?", collectionID).
		Updates(map[string]any{
			"item_count": gorm.Expr("item_count + 1"),
			"updated":    gorm.Expr("now()"),
		}).Error
}

// RemoveItem deletes a membership row and decrements item_count.
func (r *GalgameCollectionRepository) RemoveItem(tx *gorm.DB, collectionID, galgameID int) error {
	res := tx.Where("collection_id = ? AND galgame_id = ?", collectionID, galgameID).
		Delete(&model.GalgameCollectionItem{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	return tx.Model(&model.GalgameCollection{}).Where("id = ?", collectionID).
		Updates(map[string]any{
			"item_count": gorm.Expr("item_count - 1"),
			"updated":    gorm.Expr("now()"),
		}).Error
}

// GalgamesOnlyInCollection returns the galgame ids in `collectionID` that the
// user holds in NO other collection — i.e. the games whose distinct-user
// favorite_count must drop by one when this collection is deleted.
func (r *GalgameCollectionRepository) GalgamesOnlyInCollection(tx *gorm.DB, collectionID, userID int) ([]int, error) {
	ids := []int{}
	err := tx.Raw(`
		SELECT i.galgame_id
		FROM galgame_collection_item i
		WHERE i.collection_id = ?
		  AND NOT EXISTS (
		      SELECT 1 FROM galgame_collection_item j
		      WHERE j.user_id = ? AND j.galgame_id = i.galgame_id AND j.collection_id <> ?
		  )
	`, collectionID, userID, collectionID).Scan(&ids).Error
	return ids, err
}

// ── galgame_local favorite_count side-effects ──

// EnsureGalgameLocal lazy-creates the local stub so favorite_count updates land
// (a never-ingested galgame-catalogue game has no local row yet). Same rationale as
// GalgameInteractionRepository.ToggleLike.
func (r *GalgameCollectionRepository) EnsureGalgameLocal(tx *gorm.DB, galgameID int) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.GalgameLocal{ID: galgameID}).Error
}

func (r *GalgameCollectionRepository) AdjustGalgameFavoriteCount(tx *gorm.DB, galgameID, delta int) error {
	return tx.Model(&model.GalgameLocal{}).Where("id = ?", galgameID).
		Update("favorite_count", gorm.Expr("favorite_count + ?", delta)).Error
}

// DecrementFavoriteCounts drops favorite_count by one for each galgame id (bulk,
// used on collection delete). No-op on empty input.
func (r *GalgameCollectionRepository) DecrementFavoriteCounts(tx *gorm.DB, galgameIDs []int) error {
	if len(galgameIDs) == 0 {
		return nil
	}
	return tx.Model(&model.GalgameLocal{}).Where("id IN ?", galgameIDs).
		Update("favorite_count", gorm.Expr("favorite_count - 1")).Error
}

// uniqueInts de-dups a slice, preserving first-seen order.
func uniqueInts(in []int) []int {
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
