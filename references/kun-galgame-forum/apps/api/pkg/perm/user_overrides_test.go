package perm

import "testing"

// resetUserOverrides restores BOTH override layers (role + user) after a per-user
// override test, so it can never leak state into the golden matrix or Phase 2
// tests.
func resetUserOverrides(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		SetOverrides(nil)
		SetUserOverrides(nil)
	})
}

// TestCanUserGrantToRolelessUser proves a personal grant lets a plain (roleless)
// user hold a permission — the whole point of Plan A.
func TestCanUserGrantToRolelessUser(t *testing.T) {
	resetUserOverrides(t)
	SetUserOverrides(map[int][]Override{
		42: {{Permission: TopicHide, Effect: EffectGrant}},
	})

	if !CanUser(42, nil, TopicHide) {
		t.Error("granted user 42 topic.hide but CanUser = false")
	}
	// The grant is scoped to that one user and that one key.
	if CanUser(42, nil, TopicEditAny) {
		t.Error("user 42 should only hold the single granted key")
	}
	if CanUser(43, nil, TopicHide) {
		t.Error("the grant must not leak to a different user")
	}
	// The role layer is untouched: Can (no uid) still sees only role decisions.
	if Can(nil, TopicHide) {
		t.Error("a personal grant must not change the role-level Can")
	}
}

// TestCanUserRevokeBeatsRoleGrant proves a personal revoke removes a permission
// the user's role would otherwise grant.
func TestCanUserRevokeBeatsRoleGrant(t *testing.T) {
	resetUserOverrides(t)
	// A moderator holds topic.hide by baseline.
	if !CanUser(7, []string{"moderator"}, TopicHide) {
		t.Fatal("precondition: moderator should hold topic.hide")
	}
	SetUserOverrides(map[int][]Override{
		7: {{Permission: TopicHide, Effect: EffectRevoke}},
	})
	if CanUser(7, []string{"moderator"}, TopicHide) {
		t.Error("personal revoke must beat the moderator role grant")
	}
	// Other baseline keys of the role are unaffected.
	if !CanUser(7, []string{"moderator"}, TopicEditAny) {
		t.Error("revoking one key must not drop the role's other baseline keys")
	}
}

// TestCanUserRenImmunity proves a ren-holder always holds the full catalog, even
// with a personal revoke of every key — the defensive pin.
func TestCanUserRenImmunity(t *testing.T) {
	resetUserOverrides(t)
	renRevokeAll := make([]Override, 0, len(allPerms))
	for _, p := range allPerms {
		renRevokeAll = append(renRevokeAll, Override{Permission: p, Effect: EffectRevoke})
	}
	SetUserOverrides(map[int][]Override{9: renRevokeAll})

	for _, p := range allPerms {
		if !CanUser(9, []string{"ren"}, p) {
			t.Errorf("ren-holder lost %q to a personal override — ren must be immune", p)
		}
	}
	if got := len(EffectiveForUser(9, []string{"ren"})); got != totalPerms {
		t.Errorf("EffectiveForUser(ren-holder) has %d keys, want %d", got, totalPerms)
	}
}

// TestCanUserUIDZeroPassthrough proves uid <= 0 is a plain role decision — no
// personal layer is consulted (anonymous callers carry no identity).
func TestCanUserUIDZeroPassthrough(t *testing.T) {
	resetUserOverrides(t)
	// A stray override keyed at 0 must never be stored and never apply.
	SetUserOverrides(map[int][]Override{
		0: {{Permission: TopicHide, Effect: EffectGrant}},
	})
	if CanUser(0, nil, TopicHide) {
		t.Error("uid 0 must fall through to the role decision (no personal layer)")
	}
	if !CanUser(0, []string{"moderator"}, TopicHide) {
		t.Error("uid 0 with a moderator role should hold topic.hide via Can")
	}
}

// TestCanUserUnknownKeyFiltered proves a personal override naming an
// out-of-catalog key is silently dropped (never granted, never panics).
func TestCanUserUnknownKeyFiltered(t *testing.T) {
	resetUserOverrides(t)
	SetUserOverrides(map[int][]Override{
		5: {{Permission: Permission("does.not.exist"), Effect: EffectGrant}},
	})
	if CanUser(5, nil, Permission("does.not.exist")) {
		t.Error("unknown permission was granted through a personal override")
	}
	if got := len(EffectiveForUser(5, nil)); got != 0 {
		t.Errorf("roleless user 5 effective has %d keys, want 0 (unknown filtered out)", got)
	}
}

// TestCanUserReset proves SetUserOverrides(nil) clears every personal delta.
func TestCanUserReset(t *testing.T) {
	resetUserOverrides(t)
	SetUserOverrides(map[int][]Override{
		42: {{Permission: TopicHide, Effect: EffectGrant}},
		7:  {{Permission: TopicHide, Effect: EffectRevoke}},
	})
	SetUserOverrides(nil)

	if CanUser(42, nil, TopicHide) {
		t.Error("after reset the personal grant should be gone")
	}
	if !CanUser(7, []string{"moderator"}, TopicHide) {
		t.Error("after reset the personal revoke should be gone (role grant restored)")
	}
}

// TestEffectiveForUserComposition proves EffectiveForUser applies personal
// deltas on top of the role set, in catalog order.
func TestEffectiveForUserComposition(t *testing.T) {
	resetUserOverrides(t)
	SetUserOverrides(map[int][]Override{
		7: {
			{Permission: TopicHide, Effect: EffectRevoke},     // drop one baseline key
			{Permission: AdminDashboard, Effect: EffectGrant}, // add one admin-only key
		},
	})
	eff := EffectiveForUser(7, []string{"moderator"})
	// moderator baseline is 41; minus topic.hide, plus admin.dashboard = 41.
	if len(eff) != modPerms {
		t.Errorf("effective has %d keys, want %d", len(eff), modPerms)
	}
	set := make(map[Permission]bool, len(eff))
	for _, p := range eff {
		set[p] = true
	}
	if set[TopicHide] {
		t.Error("topic.hide should be revoked from the effective set")
	}
	if !set[AdminDashboard] {
		t.Error("admin.dashboard should be granted into the effective set")
	}
	assertCatalogOrder(t, eff)
}
