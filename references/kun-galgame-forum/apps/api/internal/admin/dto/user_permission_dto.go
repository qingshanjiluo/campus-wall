package dto

// UserPermissionView is the read model for the per-user permission editor
// (GET/PUT /admin/user-permissions/:uid). role_effective is the user's
// role-derived set (role baseline ± role overrides) — the reference the panel
// diffs against; effective is that set with the user's personal deltas applied.
// All permission arrays are catalog-ordered and non-null.
type UserPermissionView struct {
	UserID        int                      `json:"user_id"`
	Roles         []string                 `json:"roles"`
	RoleEffective []string                 `json:"role_effective"`
	Overrides     []UserPermissionOverride `json:"overrides"`
	Effective     []string                 `json:"effective"`
}

// UserPermissionOverride is one stored personal delta, surfaced with its audit
// stamp (updated_by / updated_at as RFC3339).
type UserPermissionOverride struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
	UpdatedBy  int    `json:"updated_by"`
	UpdatedAt  string `json:"updated_at"`
}

// ReplaceUserOverridesRequest is the PUT body — the FULL desired personal
// override set for the path user. It REPLACES that user's current overrides
// atomically; an empty list resets them to their role-derived set. It reuses
// ReplaceOverrideItem (the role editor's item shape).
type ReplaceUserOverridesRequest struct {
	Overrides []ReplaceOverrideItem `json:"overrides"`
}

// MyPermissionsResponse feeds FE visibility only — it lists the CURRENT user's
// effective permissions so the UI can show/hide affordances; it grants nothing.
type MyPermissionsResponse struct {
	Permissions []string `json:"permissions"`
}
