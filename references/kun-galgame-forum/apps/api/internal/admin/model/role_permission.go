package model

import "time"

// RolePermissionOverride is a single DEVIATION from the compiled permission
// baseline (pkg/perm): for one (role, permission) pair, effect 'grant' adds a key
// the role's baseline lacks and 'revoke' removes one it holds. The table stores
// ONLY deltas — an empty table means every role holds exactly its compiled
// baseline, so effective(role) = (baseline ∪ grants) − revokes. `ren` is pinned
// to the full catalog and is never stored here.
type RolePermissionOverride struct {
	Role       string    `gorm:"column:role;primaryKey" json:"role"`
	Permission string    `gorm:"column:permission;primaryKey" json:"permission"`
	Effect     string    `gorm:"column:effect;not null" json:"effect"`
	UpdatedBy  int       `gorm:"column:updated_by;not null;default:0" json:"updated_by"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RolePermissionOverride) TableName() string { return "role_permission_override" }
