package model

import "time"

// UserPermissionOverride is a single DEVIATION from a user's ROLE-DERIVED
// effective set (migration 063): for one (user_id, permission) pair, effect
// 'grant' adds a key their roles do not grant and 'revoke' removes one their
// roles would grant. Personal deltas (Phase 3 / Plan A) let even a roleless user
// hold a permission, and a personal revoke beats a role grant. So
// effective(user) = ((role baseline ± role overrides) ∪ personal grants) −
// personal revokes. A user holding `ren` is pinned to the full catalog and is
// never stored here.
type UserPermissionOverride struct {
	UserID     int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	Permission string    `gorm:"column:permission;primaryKey" json:"permission"`
	Effect     string    `gorm:"column:effect;not null" json:"effect"`
	UpdatedBy  int       `gorm:"column:updated_by;not null;default:0" json:"updated_by"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (UserPermissionOverride) TableName() string { return "user_permission_override" }
