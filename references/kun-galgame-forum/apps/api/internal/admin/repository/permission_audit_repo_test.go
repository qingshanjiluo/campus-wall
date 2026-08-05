package repository

import (
	"testing"

	"kun-galgame-api/internal/admin/model"
)

// TestBuildAuditRowReplace proves a non-empty new set is classified 'replace' and
// the before/after delta sets are captured verbatim.
func TestBuildAuditRowReplace(t *testing.T) {
	before := []model.AuditDelta{{Permission: "topic.hide", Effect: "grant"}}
	after := []model.AuditDelta{{Permission: "topic.hide", Effect: "revoke"}}
	row := buildAuditRow(7, "user", "4242", before, after)

	if row.Action != "replace" {
		t.Errorf("action = %q, want replace", row.Action)
	}
	if row.OperatorID != 7 || row.SubjectKind != "user" || row.Subject != "4242" {
		t.Errorf("row header mismatch: %+v", row)
	}
	if len(row.BeforeRows) != 1 || row.BeforeRows[0].Effect != "grant" {
		t.Errorf("before rows not captured: %+v", row.BeforeRows)
	}
	if len(row.AfterRows) != 1 || row.AfterRows[0].Effect != "revoke" {
		t.Errorf("after rows not captured: %+v", row.AfterRows)
	}
}

// TestBuildAuditRowReset proves an EMPTY new set is classified 'reset' and a nil
// before slice is stored as a non-nil empty slice (jsonb [] not null).
func TestBuildAuditRowReset(t *testing.T) {
	row := buildAuditRow(1, "role", "moderator", nil, nil)
	if row.Action != "reset" {
		t.Errorf("action = %q, want reset (empty after set)", row.Action)
	}
	if row.BeforeRows == nil || row.AfterRows == nil {
		t.Error("before/after must be non-nil so jsonb stores [] not null")
	}
	if len(row.BeforeRows) != 0 || len(row.AfterRows) != 0 {
		t.Errorf("empty reset should have empty delta sets, got %+v / %+v", row.BeforeRows, row.AfterRows)
	}
}

// TestRowsToDeltas proves both projections drop timestamps and keep only
// {permission, effect}.
func TestRowsToDeltas(t *testing.T) {
	roleDeltas := roleRowsToDeltas([]model.RolePermissionOverride{
		{Role: "admin", Permission: "user.purge_content", Effect: "revoke"},
	})
	if len(roleDeltas) != 1 || roleDeltas[0].Permission != "user.purge_content" || roleDeltas[0].Effect != "revoke" {
		t.Errorf("roleRowsToDeltas = %+v", roleDeltas)
	}
	userDeltas := userRowsToDeltas([]model.UserPermissionOverride{
		{UserID: 42, Permission: "topic.hide", Effect: "grant"},
	})
	if len(userDeltas) != 1 || userDeltas[0].Permission != "topic.hide" || userDeltas[0].Effect != "grant" {
		t.Errorf("userRowsToDeltas = %+v", userDeltas)
	}
	// Empty input yields a non-nil empty slice.
	if got := userRowsToDeltas(nil); got == nil {
		t.Error("userRowsToDeltas(nil) should be non-nil empty")
	}
}
