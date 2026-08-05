package perm

import (
	"slices"
	"sync/atomic"
)

// This file adds the Phase 3 PER-USER override layer on top of the role layer
// (overrides.go). Where role overrides deviate a whole role's bundle, personal
// overrides deviate ONE user: a personal grant lets even a roleless plain user
// hold a permission, and a personal revoke removes a permission a user's roles
// would otherwise grant. The deltas are persisted in Postgres (migration 063)
// and pushed here via SetUserOverrides. The package stays dependency-free — it
// only owns the set algebra; storage + HTTP live in internal/admin.
//
// Resolution (the fixed semantics):
//
//	effective(user) = ((role baseline ± role overrides) ∪ personal grants)
//	                   − personal revokes
//
// with one exception: a user whose roles include `ren` always holds the FULL
// catalog — CanUser pins ren-holders defensively so no override can reduce them.

// userPerms is one user's compiled personal delta: the keys they were granted
// and the keys they were revoked (both already filtered to the known catalog).
type userPerms struct {
	grants  map[Permission]struct{}
	revokes map[Permission]struct{}
}

// userOverrideTable is the live per-user delta table, swapped atomically by
// SetUserOverrides so CanUser can read it lock-free while a refresh installs a
// new one. A user with no personal deltas is simply absent from the map.
type userOverrideTable struct {
	users map[int]*userPerms
}

func (t *userOverrideTable) forUID(uid int) *userPerms {
	if t == nil {
		return nil
	}
	return t.users[uid]
}

// userCurrent is the live per-user override table (empty until Load populates it).
var userCurrent atomic.Pointer[userOverrideTable]

func init() {
	userCurrent.Store(&userOverrideTable{users: map[int]*userPerms{}})
}

// SetUserOverrides recomputes the per-user delta table from the given rows and
// swaps it in atomically. Unknown permission keys, non-grant/revoke effects, and
// non-positive uids are dropped defensively (a single cheap choke point, mirror
// of SetOverrides). Passing nil/empty clears all personal overrides.
func SetUserOverrides(overrides map[int][]Override) {
	tbl := &userOverrideTable{users: make(map[int]*userPerms, len(overrides))}
	for uid, ovs := range overrides {
		if uid <= 0 {
			continue
		}
		up := &userPerms{grants: map[Permission]struct{}{}, revokes: map[Permission]struct{}{}}
		for _, ov := range ovs {
			if _, known := catalogIndex[ov.Permission]; !known {
				continue
			}
			switch ov.Effect {
			case EffectGrant:
				up.grants[ov.Permission] = struct{}{}
			case EffectRevoke:
				up.revokes[ov.Permission] = struct{}{}
			}
		}
		if len(up.grants) == 0 && len(up.revokes) == 0 {
			continue
		}
		tbl.users[uid] = up
	}
	userCurrent.Store(tbl)
}

// CanUser is THE enforcement entry point for a concrete caller: it layers the
// caller's personal overrides on top of the role decision. Order:
//
//   - uid <= 0 (anonymous / no identity) → plain role decision Can(roles, p).
//   - roles include `ren` → the full catalog (IsKnownPermission), pinned
//     defensively so no personal or role override can lock a ren-holder out.
//   - a personal REVOKE of p → false (beats any role grant).
//   - a personal GRANT of p → true (works even for a roleless plain user).
//   - otherwise → the role decision Can(roles, p).
//
// Every pure-forum enforcement point calls CanUser; Can is the role-level
// primitive underneath it (used by bundles / tests / the admin matrix).
func CanUser(uid int, roles []string, p Permission) bool {
	if uid <= 0 {
		return Can(roles, p)
	}
	if slices.Contains(roles, roleRen) {
		return IsKnownPermission(p)
	}
	if up := userCurrent.Load().forUID(uid); up != nil {
		if _, revoked := up.revokes[p]; revoked {
			return false
		}
		if _, granted := up.grants[p]; granted {
			return true
		}
	}
	return Can(roles, p)
}

// EffectiveForUser returns the caller's full effective permission set in catalog
// order (a non-nil slice) — the role-derived set with the caller's personal
// deltas applied, exactly as CanUser would decide each key. Feeds /perm/mine and
// the per-user admin panel.
func EffectiveForUser(uid int, roles []string) []Permission {
	out := make([]Permission, 0, len(catalog))
	for _, p := range catalog {
		if CanUser(uid, roles, p) {
			out = append(out, p)
		}
	}
	return out
}
