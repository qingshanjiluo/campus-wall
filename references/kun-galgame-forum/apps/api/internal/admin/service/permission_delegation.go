package service

import (
	"kun-galgame-api/internal/admin/dto"
	"kun-galgame-api/internal/admin/model"
	"kun-galgame-api/pkg/perm"
)

// This file holds the POSSESSION guard shared by the role and per-user permission
// editors (delegation rule, Change 2). An operator may only add, remove, or change
// override rows whose permission key is in the operator's OWN current effective
// set. Because a PUT carries the FULL replacement override set, the guard is
// applied ONLY to the true delta — the keys whose stored effect differs from the
// submitted effect. Rows carried over unchanged are NOT edits and pass untouched
// (e.g. a row a higher-ranked operator set earlier survives when a lower-ranked
// operator saves an unrelated change). The RANK half of the rule lives in
// pkg/perm (perm.Rank / perm.RoleRank).

// operatorEffectiveSet returns the operator's OWN current effective permission set
// (role layer + their personal overrides), as a membership set. A ren operator
// holds the full catalog, so the possession rule never bites them.
func operatorEffectiveSet(operatorUID int, operatorRoles []string) map[perm.Permission]bool {
	set := make(map[perm.Permission]bool)
	for _, p := range perm.EffectiveForUser(operatorUID, operatorRoles) {
		set[p] = true
	}
	return set
}

// possessionOffender returns the FIRST changed key (in catalog order, for a stable
// message) whose override differs between the stored and submitted sets but is NOT
// held by the operator. ok=true means every changed row is within the operator's
// own powers. effect strings are "grant"/"revoke" (never ""), so an absent key
// reads as "" and compares unequal to any real effect — exactly the added/removed
// signal; identical effects (or both absent) count as carried-over and are skipped.
func possessionOffender(stored, submitted map[string]string, holds map[perm.Permission]bool) (string, bool) {
	for _, p := range perm.Catalog() {
		key := string(p)
		if stored[key] == submitted[key] {
			continue
		}
		if !holds[p] {
			return key, false
		}
	}
	return "", true
}

// effectMapFromItems collapses a PUT's items into a (permission → effect) lookup.
func effectMapFromItems(items []dto.ReplaceOverrideItem) map[string]string {
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Permission] = it.Effect
	}
	return m
}

// effectMapFromRoleRows collapses the stored override rows of ONE role into a
// (permission → effect) lookup (other roles' rows are ignored).
func effectMapFromRoleRows(rows []model.RolePermissionOverride, role string) map[string]string {
	m := make(map[string]string)
	for _, r := range rows {
		if r.Role == role {
			m[r.Permission] = r.Effect
		}
	}
	return m
}

// effectMapFromUserRows collapses one user's stored personal override rows into a
// (permission → effect) lookup.
func effectMapFromUserRows(rows []model.UserPermissionOverride) map[string]string {
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Permission] = r.Effect
	}
	return m
}
