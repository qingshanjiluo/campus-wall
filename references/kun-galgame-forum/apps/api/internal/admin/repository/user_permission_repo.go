package repository

import (
	"context"
	"strconv"
	"time"

	"kun-galgame-api/internal/admin/model"

	"gorm.io/gorm"
)

// UserPermissionRepository persists the per-user permission override deltas
// (migration 063) and writes the audit trail (migration 064) atomically with
// each replace. The table is tiny (only deviations), so a full scan is cheap.
type UserPermissionRepository struct {
	db *gorm.DB
}

func NewUserPermissionRepository(db *gorm.DB) *UserPermissionRepository {
	return &UserPermissionRepository{db: db}
}

// ListAll returns every per-user override row across all users, in a stable order
// (the boot/refresher Load consumes this to populate the pkg/perm user layer).
func (r *UserPermissionRepository) ListAll(ctx context.Context) ([]model.UserPermissionOverride, error) {
	var rows []model.UserPermissionOverride
	err := r.db.WithContext(ctx).Order("user_id ASC, permission ASC").Find(&rows).Error
	return rows, err
}

// ListForUser returns one user's override rows, in a stable order.
func (r *UserPermissionRepository) ListForUser(ctx context.Context, userID int) ([]model.UserPermissionOverride, error) {
	var rows []model.UserPermissionOverride
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("permission ASC").Find(&rows).Error
	return rows, err
}

// ReplaceForUser atomically replaces one user's ENTIRE override set AND writes one
// audit row — in a single transaction, so a partial replace can never be observed
// and the audit trail can never drift from the change it records. The before rows
// are captured with a SELECT inside the tx BEFORE the delete. An empty rows slice
// resets the user to their role-derived set (delete-only; audit action 'reset').
func (r *UserPermissionRepository) ReplaceForUser(ctx context.Context, userID int, rows []model.UserPermissionOverride, operatorUID int) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before []model.UserPermissionOverride
		if err := tx.Where("user_id = ?", userID).Order("permission ASC").Find(&before).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserPermissionOverride{}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].UserID = userID
			rows[i].UpdatedBy = operatorUID
			rows[i].UpdatedAt = now
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return writeAudit(tx, operatorUID, "user", strconv.Itoa(userID),
			userRowsToDeltas(before), userRowsToDeltas(rows))
	})
}

// userRowsToDeltas projects override rows into the compact {permission, effect}
// audit shape (no timestamps).
func userRowsToDeltas(rows []model.UserPermissionOverride) []model.AuditDelta {
	out := make([]model.AuditDelta, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.AuditDelta{Permission: r.Permission, Effect: r.Effect})
	}
	return out
}
