package repository

import (
	"context"
	"time"

	"kun-galgame-api/internal/admin/model"

	"gorm.io/gorm"
)

// RolePermissionRepository persists the role→permission override deltas
// (migration 062). The table is tiny (only deviations from the compiled
// baseline), so a full scan is cheap.
type RolePermissionRepository struct {
	db *gorm.DB
}

func NewRolePermissionRepository(db *gorm.DB) *RolePermissionRepository {
	return &RolePermissionRepository{db: db}
}

// ListAll returns every override row across all roles, in a stable order.
func (r *RolePermissionRepository) ListAll(ctx context.Context) ([]model.RolePermissionOverride, error) {
	var rows []model.RolePermissionOverride
	err := r.db.WithContext(ctx).Order("role ASC, permission ASC").Find(&rows).Error
	return rows, err
}

// ReplaceForRole atomically replaces one role's ENTIRE override set AND writes one
// audit row: capture the before rows (SELECT inside the tx, before the delete),
// delete all existing rows, insert the new set stamped with the operator + now,
// then append the audit row. One transaction, so a partial replace can never be
// observed and the audit trail can never drift from the change. An empty rows
// slice resets the role to its pure baseline (delete-only; audit action 'reset').
func (r *RolePermissionRepository) ReplaceForRole(ctx context.Context, role string, rows []model.RolePermissionOverride, operatorUID int) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before []model.RolePermissionOverride
		if err := tx.Where("role = ?", role).Order("permission ASC").Find(&before).Error; err != nil {
			return err
		}
		if err := tx.Where("role = ?", role).Delete(&model.RolePermissionOverride{}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].Role = role
			rows[i].UpdatedBy = operatorUID
			rows[i].UpdatedAt = now
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return writeAudit(tx, operatorUID, "role", role,
			roleRowsToDeltas(before), roleRowsToDeltas(rows))
	})
}

// roleRowsToDeltas projects override rows into the compact {permission, effect}
// audit shape (no timestamps).
func roleRowsToDeltas(rows []model.RolePermissionOverride) []model.AuditDelta {
	out := make([]model.AuditDelta, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.AuditDelta{Permission: r.Permission, Effect: r.Effect})
	}
	return out
}
