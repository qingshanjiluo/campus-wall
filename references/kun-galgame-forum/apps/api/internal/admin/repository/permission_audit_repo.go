package repository

import (
	"context"

	"kun-galgame-api/internal/admin/model"

	"gorm.io/gorm"
)

// writeAudit appends one audit row inside the given transaction. Both the role
// replace (ReplaceForRole) and the user replace (ReplaceForUser) call it with the
// before/after delta sets they captured, so every override change leaves a
// tamper-evident trail atomically with the change itself.
func writeAudit(tx *gorm.DB, operatorID int, subjectKind, subject string, before, after []model.AuditDelta) error {
	row := buildAuditRow(operatorID, subjectKind, subject, before, after)
	return tx.Create(&row).Error
}

// buildAuditRow is the PURE construction of an audit row (no DB) so the
// before/after capture and the action classification are unit-testable. action
// is 'reset' when the new set is empty, else 'replace'; before/after are forced
// non-nil so the jsonb column stores [] rather than the JSON null a nil slice
// would serialize to.
func buildAuditRow(operatorID int, subjectKind, subject string, before, after []model.AuditDelta) model.PermissionAuditLog {
	action := "replace"
	if len(after) == 0 {
		action = "reset"
	}
	return model.PermissionAuditLog{
		OperatorID:  operatorID,
		SubjectKind: subjectKind,
		Subject:     subject,
		Action:      action,
		BeforeRows:  nonNilDeltas(before),
		AfterRows:   nonNilDeltas(after),
	}
}

func nonNilDeltas(d []model.AuditDelta) []model.AuditDelta {
	if d == nil {
		return []model.AuditDelta{}
	}
	return d
}

// PermissionAuditRepository serves the append-only audit log read (newest first,
// paginated). Writes happen only through writeAudit inside the override replace
// transactions — this repo never mutates the table.
type PermissionAuditRepository struct {
	db *gorm.DB
}

func NewPermissionAuditRepository(db *gorm.DB) *PermissionAuditRepository {
	return &PermissionAuditRepository{db: db}
}

// List returns one page of audit rows (newest first) plus the total row count.
// offset/limit are already sanitized by the service. Ordering by created_at DESC
// (the indexed column) with id DESC as the tie-breaker gives a stable newest-first
// page.
func (r *PermissionAuditRepository) List(ctx context.Context, offset, limit int) ([]model.PermissionAuditLog, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.PermissionAuditLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.PermissionAuditLog
	err := r.db.WithContext(ctx).Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}
