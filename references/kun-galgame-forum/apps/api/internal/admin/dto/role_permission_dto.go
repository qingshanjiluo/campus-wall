package dto

// RolePermissionMatrix is the full read model for the admin role→permission
// editor (GET /admin/role-permissions). Catalog lists every overridable key in
// canonical order; Roles maps each manageable role (+ the pinned ren) to its
// compiled baseline, its stored overrides, and its computed effective set. All
// arrays are in catalog order.
type RolePermissionMatrix struct {
	Catalog []string                      `json:"catalog"`
	Roles   map[string]RolePermissionRole `json:"roles"`
}

// RolePermissionRole is one role's row in the matrix.
type RolePermissionRole struct {
	Baseline  []string                 `json:"baseline"`
	Overrides []RolePermissionOverride `json:"overrides"`
	Effective []string                 `json:"effective"`
	// Locked marks the pinned ren role (full catalog, not editable). Omitted for
	// the three manageable roles so the wire matches the contract exactly.
	Locked bool `json:"locked,omitempty"`
}

// RolePermissionOverride is one stored delta, surfaced in the matrix with its
// audit stamp (updated_by / updated_at as RFC3339).
type RolePermissionOverride struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
	UpdatedBy  int    `json:"updated_by"`
	UpdatedAt  string `json:"updated_at"`
}

// ReplaceOverridesRequest is the PUT body — the FULL desired override set for the
// path role. It REPLACES that role's current overrides atomically; an empty list
// resets the role to its pure baseline.
type ReplaceOverridesRequest struct {
	Overrides []ReplaceOverrideItem `json:"overrides"`
}

// ReplaceOverrideItem is one requested delta in a PUT body.
type ReplaceOverrideItem struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
}
