package perm

import (
	"slices"
	"sync/atomic"
)

// This file adds the Phase 2 runtime override layer on top of the compiled
// baseline in perm.go. An admin can grant/revoke individual permission keys per
// role at runtime; the deltas are persisted in Postgres and pushed here via
// SetOverrides. The package stays dependency-free (no gorm/fiber) — it only
// owns the set algebra; storage + HTTP live in internal/admin.

// Effect values for a single override row.
const (
	// EffectGrant adds a permission a role's baseline lacks.
	EffectGrant = "grant"
	// EffectRevoke removes a permission a role's baseline holds.
	EffectRevoke = "revoke"
)

// roleRen is the one role pinned to the full catalog. It is never stored as an
// override and is ignored defensively in the apply path — overrides must never
// be able to reduce ren below every permission.
const roleRen = "ren"

// Override is a single deviation from a role's compiled baseline.
type Override struct {
	Permission Permission
	Effect     string // EffectGrant | EffectRevoke
}

// catalog is the full ordered permission vocabulary — every overridable key in
// declaration order. adminPerms IS that list by construction (admin holds
// everything: the 45 moderator keys plus the two admin-only keys), so we derive
// catalog from it rather than maintain a third hand-copied list.
var catalog = append([]Permission{}, adminPerms...)

// catalogIndex maps each known permission to its position in catalog — used for
// O(1) membership tests (unknown-key filtering) and stable ordering.
var catalogIndex = func() map[Permission]int {
	m := make(map[Permission]int, len(catalog))
	for i, p := range catalog {
		m[p] = i
	}
	return m
}()

// current is the live effective resolver, swapped atomically by SetOverrides so
// Can can read it lock-free while a refresh installs a new table.
var current atomic.Pointer[resolverT]

func init() {
	// Start at the pure baseline (no overrides) so behavior matches Phase 1 until
	// the override table is loaded.
	current.Store(buildResolver(nil))
}

// SetOverrides recomputes the effective role→permission table from the compiled
// baseline plus the given per-role overrides and swaps it in atomically. Unknown
// permission keys and any "ren" entry are ignored defensively (a single cheap
// choke point). Passing nil/empty restores the pure baseline.
func SetOverrides(overrides map[string][]Override) {
	current.Store(buildResolver(overrides))
}

// buildResolver produces the effective resolver: for every role that could hold
// a permission (the baseline bundle roles + any role named in overrides + ren),
// it applies that role's overrides to its baseline. ren is always pinned to the
// full catalog regardless of any stored rows.
func buildResolver(overrides map[string][]Override) *resolverT {
	roles := map[string]struct{}{roleRen: {}}
	for r := range Bundles {
		roles[r] = struct{}{}
	}
	for r := range overrides {
		roles[r] = struct{}{}
	}
	grants := make(map[string]map[Permission]struct{}, len(roles))
	for r := range roles {
		grants[r] = applyOverrides(r, overrides[r])
	}
	return &resolverT{grants: grants}
}

// applyOverrides computes one role's effective set: (baseline ∪ grants) − revokes.
// ren short-circuits to the full catalog (its overrides are ignored). Unknown
// permissions and non-grant/revoke effects are skipped defensively — writes
// validate first, so this only guards against stale/hand-edited rows.
func applyOverrides(role string, overrides []Override) map[Permission]struct{} {
	if role == roleRen {
		return fullCatalogSet()
	}
	baseline := Bundles[role]
	set := make(map[Permission]struct{}, len(baseline)+len(overrides))
	for _, p := range baseline {
		set[p] = struct{}{}
	}
	for _, ov := range overrides {
		if _, known := catalogIndex[ov.Permission]; !known {
			continue
		}
		switch ov.Effect {
		case EffectGrant:
			set[ov.Permission] = struct{}{}
		case EffectRevoke:
			delete(set, ov.Permission)
		}
	}
	return set
}

// fullCatalogSet returns a fresh set of every permission (ren's pinned grant).
func fullCatalogSet() map[Permission]struct{} {
	set := make(map[Permission]struct{}, len(catalog))
	for _, p := range catalog {
		set[p] = struct{}{}
	}
	return set
}

// orderedFrom returns the permissions of set in catalog order, as a non-nil
// slice (an empty set yields []Permission{}, so JSON renders [] not null).
func orderedFrom(set map[Permission]struct{}) []Permission {
	out := make([]Permission, 0, len(set))
	for _, p := range catalog {
		if _, ok := set[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Catalog returns the 47 permission keys in declaration order (a copy).
func Catalog() []Permission {
	return append([]Permission{}, catalog...)
}

// IsKnownPermission reports whether p is one of the overridable catalog keys.
func IsKnownPermission(p Permission) bool {
	_, ok := catalogIndex[p]
	return ok
}

// Baseline returns a role's COMPILED baseline set in catalog order (a copy),
// ignoring any runtime overrides. Empty for creator/user/unknown; all 47 for
// admin/ren.
func Baseline(role string) []Permission {
	set := make(map[Permission]struct{}, len(Bundles[role]))
	for _, p := range Bundles[role] {
		set[p] = struct{}{}
	}
	return orderedFrom(set)
}

// BaselineHas reports whether a role's compiled baseline holds p — the seam the
// write path uses to reject no-op overrides (granting a baseline key, or
// revoking a non-baseline key).
func BaselineHas(role string, p Permission) bool {
	return slices.Contains(Bundles[role], p)
}

// EffectiveSet computes a role's effective permission set from its baseline and
// the given overrides — (baseline ∪ grants) − revokes — in catalog order. It is
// PURE (independent of the installed global table), so the write path can check
// a prospective state (e.g. the moderator ⊆ admin containment invariant) before
// persisting. ren is pinned to the full catalog.
func EffectiveSet(role string, overrides []Override) []Permission {
	return orderedFrom(applyOverrides(role, overrides))
}

// EffectiveBundles returns the CURRENT effective sets (baseline + installed
// overrides) for creator/moderator/admin/ren, each in catalog order. creator is
// always present (empty [] when it has no grants); ren is the full catalog.
func EffectiveBundles() map[string][]Permission {
	r := current.Load()
	roles := []string{"creator", "moderator", "admin", roleRen}
	out := make(map[string][]Permission, len(roles))
	for _, role := range roles {
		out[role] = orderedFrom(r.grants[role])
	}
	return out
}
