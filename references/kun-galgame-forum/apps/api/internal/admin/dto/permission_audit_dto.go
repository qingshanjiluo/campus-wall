package dto

// PermissionAuditPage is the paginated audit read (GET /admin/permission-audit),
// newest first. Users maps operator ids (as decimal-string keys) to display info
// for attribution — best-effort (an OAuth batch failure yields an empty map).
type PermissionAuditPage struct {
	Total int64                          `json:"total"`
	Items []PermissionAuditItem          `json:"items"`
	Users map[string]PermissionAuditUser `json:"users"`
}

// PermissionAuditItem is one audit row. before/after are the compact delta sets
// ({permission, effect}) captured at replace time; created_at is RFC3339.
type PermissionAuditItem struct {
	ID          int64                `json:"id"`
	OperatorID  int                  `json:"operator_id"`
	SubjectKind string               `json:"subject_kind"`
	Subject     string               `json:"subject"`
	Action      string               `json:"action"`
	Before      []PermissionAuditRow `json:"before"`
	After       []PermissionAuditRow `json:"after"`
	CreatedAt   string               `json:"created_at"`
}

// PermissionAuditRow is one {permission, effect} pair in a before/after set.
type PermissionAuditRow struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
}

// PermissionAuditUser is the operator display shape (best-effort attribution).
type PermissionAuditUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}
