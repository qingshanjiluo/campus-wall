package model

import "time"

// PermissionAuditLog is one append-only entry in the permission-override audit
// trail (migration 064). It is written IN THE SAME TRANSACTION as the override
// replace it records, capturing the before/after delta sets. subject_kind is
// 'role' | 'user'; subject is the role name or the target uid as a decimal
// string; action is 'reset' (new set empty) | 'replace'. before/after are stored
// as compact jsonb arrays of {permission, effect} via GORM's json serializer.
type PermissionAuditLog struct {
	ID          int64        `gorm:"column:id;primaryKey" json:"id"`
	OperatorID  int          `gorm:"column:operator_id" json:"operator_id"`
	SubjectKind string       `gorm:"column:subject_kind" json:"subject_kind"`
	Subject     string       `gorm:"column:subject" json:"subject"`
	Action      string       `gorm:"column:action" json:"action"`
	BeforeRows  []AuditDelta `gorm:"column:before_rows;serializer:json;type:jsonb;default:'[]'" json:"before_rows"`
	AfterRows   []AuditDelta `gorm:"column:after_rows;serializer:json;type:jsonb;default:'[]'" json:"after_rows"`
	CreatedAt   time.Time    `gorm:"column:created_at" json:"created_at"`
}

func (PermissionAuditLog) TableName() string { return "permission_audit_log" }

// AuditDelta is one {permission, effect} pair inside an audit row's before/after
// arrays — compact (no timestamps).
type AuditDelta struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
}
