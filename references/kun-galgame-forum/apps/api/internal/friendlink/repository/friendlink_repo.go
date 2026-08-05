package repository

import (
	"kun-galgame-api/internal/friendlink/model"

	"gorm.io/gorm"
)

type FriendLinkRepository struct {
	db *gorm.DB
}

func NewFriendLinkRepository(db *gorm.DB) *FriendLinkRepository {
	return &FriendLinkRepository{db: db}
}

// FindAllOrdered returns every friend link ordered by category then sort_order
// (the order each group renders in). Cheap: backed by idx_friend_link_category_order.
func (r *FriendLinkRepository) FindAllOrdered() []model.FriendLink {
	var links []model.FriendLink
	r.db.Order("category ASC, sort_order ASC, id ASC").Find(&links)
	return links
}

// Create appends the link to the END of its category (sort_order = max+1) so a
// new link shows up last and existing positions are untouched.
func (r *FriendLinkRepository) Create(fl *model.FriendLink) error {
	var maxOrder int
	r.db.Model(&model.FriendLink{}).
		Where("category = ?", fl.Category).
		Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder)
	fl.SortOrder = maxOrder + 1
	return r.db.Create(fl).Error
}

// Update patches the given columns on a link by id.
func (r *FriendLinkRepository) Update(id int, fields map[string]any) error {
	return r.db.Model(&model.FriendLink{}).Where("id = ?", id).Updates(fields).Error
}

// Delete removes a link by id.
func (r *FriendLinkRepository) Delete(id int) {
	r.db.Delete(&model.FriendLink{}, id)
}

// Reorder rewrites sort_order for every id to its index in the provided slice,
// in one transaction. The `AND category = ?` guard means a stray id (wrong
// category / already deleted) is silently skipped rather than mis-ordered.
func (r *FriendLinkRepository) Reorder(category string, ids []int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&model.FriendLink{}).
				Where("id = ? AND category = ?", id, category).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
