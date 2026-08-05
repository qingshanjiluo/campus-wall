package perm

import "testing"

// resetOverrides registers a cleanup that restores the pure baseline, so an
// override test can never leak state into the golden matrix tests (perm_test.go).
func resetOverrides(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { SetOverrides(nil) })
}

// TestOverridesApply proves a grant and a revoke both flow through Can: granting
// a moderation key to creator (which holds nothing) makes it true, and revoking
// one from moderator (which holds all 41) makes it false.
func TestOverridesApply(t *testing.T) {
	resetOverrides(t)
	SetOverrides(map[string][]Override{
		"creator":   {{Permission: TopicHide, Effect: EffectGrant}},
		"moderator": {{Permission: TopicHide, Effect: EffectRevoke}},
	})

	if !Can([]string{"creator"}, TopicHide) {
		t.Error("granted creator topic.hide but Can = false")
	}
	if Can([]string{"moderator"}, TopicHide) {
		t.Error("revoked moderator topic.hide but Can = true")
	}
	// A grant is scoped to the one role: creator's other keys stay ungranted, and
	// moderator's other keys are untouched.
	if Can([]string{"creator"}, TopicEditAny) {
		t.Error("creator should only hold the single granted key")
	}
	if !Can([]string{"moderator"}, TopicEditAny) {
		t.Error("revoking one key must not drop moderator's other baseline keys")
	}
}

// TestRenImmunity proves ren is pinned to the full catalog: even a revoke-every-key
// override for ren is ignored, so ren still grants all 47.
func TestRenImmunity(t *testing.T) {
	resetOverrides(t)
	renRevokeAll := make([]Override, 0, len(allPerms))
	for _, p := range allPerms {
		renRevokeAll = append(renRevokeAll, Override{Permission: p, Effect: EffectRevoke})
	}
	SetOverrides(map[string][]Override{"ren": renRevokeAll})

	for _, p := range allPerms {
		if !Can([]string{"ren"}, p) {
			t.Errorf("ren lost %q to an override — ren must be immune", p)
		}
	}
	if got := len(EffectiveBundles()["ren"]); got != totalPerms {
		t.Errorf("effective ren has %d keys, want %d (pinned to full catalog)", got, totalPerms)
	}
}

// TestUnknownKeyFiltered proves an override naming a permission outside the
// catalog is silently dropped in the apply path (never granted, never panics).
func TestUnknownKeyFiltered(t *testing.T) {
	resetOverrides(t)
	SetOverrides(map[string][]Override{
		"creator": {{Permission: Permission("does.not.exist"), Effect: EffectGrant}},
	})
	if Can([]string{"creator"}, Permission("does.not.exist")) {
		t.Error("unknown permission was granted through an override")
	}
	if got := len(EffectiveBundles()["creator"]); got != 0 {
		t.Errorf("creator effective has %d keys, want 0 (unknown filtered out)", got)
	}
}

// TestNilResetRestoresBaseline proves SetOverrides(nil) reverts to the compiled
// baseline after any set of overrides.
func TestNilResetRestoresBaseline(t *testing.T) {
	resetOverrides(t)
	SetOverrides(map[string][]Override{
		"creator":   {{Permission: TopicHide, Effect: EffectGrant}},
		"moderator": {{Permission: TopicHide, Effect: EffectRevoke}},
	})
	SetOverrides(nil)

	if Can([]string{"creator"}, TopicHide) {
		t.Error("after reset creator should hold nothing")
	}
	if !Can([]string{"moderator"}, TopicHide) {
		t.Error("after reset moderator should hold its baseline topic.hide again")
	}
	if got := len(EffectiveBundles()["moderator"]); got != modPerms {
		t.Errorf("after reset moderator has %d keys, want %d", got, modPerms)
	}
}

// TestEffectiveBundlesShape pins the pure-baseline bundle shape: creator empty,
// moderator 45, admin 47, ren 47 — all in catalog order.
func TestEffectiveBundlesShape(t *testing.T) {
	resetOverrides(t)
	SetOverrides(nil)
	b := EffectiveBundles()
	sizes := map[string]int{"creator": 0, "moderator": modPerms, "admin": totalPerms, "ren": totalPerms}
	for role, want := range sizes {
		got, ok := b[role]
		if !ok {
			t.Errorf("EffectiveBundles missing role %q", role)
			continue
		}
		if len(got) != want {
			t.Errorf("effective %q has %d keys, want %d", role, len(got), want)
		}
	}
	// creator must be a non-nil empty slice (JSON [] not null).
	if b["creator"] == nil {
		t.Error("creator bundle is nil, want an empty non-nil slice")
	}
	assertCatalogOrder(t, b["admin"])
}

// TestCatalogAndBaseline pins Catalog() size and the per-role Baseline() sizes.
func TestCatalogAndBaseline(t *testing.T) {
	if got := len(Catalog()); got != totalPerms {
		t.Errorf("Catalog() has %d keys, want %d", got, totalPerms)
	}
	cases := map[string]int{"creator": 0, "moderator": modPerms, "admin": totalPerms, "ren": totalPerms, "user": 0, "unknown": 0}
	for role, want := range cases {
		if got := len(Baseline(role)); got != want {
			t.Errorf("Baseline(%q) has %d keys, want %d", role, got, want)
		}
	}
	if !BaselineHas("moderator", TopicHide) {
		t.Error("BaselineHas(moderator, topic.hide) = false, want true")
	}
	if BaselineHas("moderator", AdminDashboard) {
		t.Error("BaselineHas(moderator, admin.dashboard) = true, want false (admin-only)")
	}
	if BaselineHas("creator", TopicHide) {
		t.Error("BaselineHas(creator, topic.hide) = true, want false")
	}
}

// TestEffectiveSetPure proves EffectiveSet computes a prospective set without
// touching the installed global table — grant onto creator, revoke off moderator,
// and ren pinned regardless of overrides.
func TestEffectiveSetPure(t *testing.T) {
	if got := len(EffectiveSet("creator", []Override{{Permission: TopicHide, Effect: EffectGrant}})); got != 1 {
		t.Errorf("EffectiveSet(creator, +topic.hide) has %d keys, want 1", got)
	}
	if got := len(EffectiveSet("moderator", []Override{{Permission: TopicHide, Effect: EffectRevoke}})); got != modPerms-1 {
		t.Errorf("EffectiveSet(moderator, -topic.hide) has %d keys, want %d", got, modPerms-1)
	}
	if got := len(EffectiveSet("ren", []Override{{Permission: TopicHide, Effect: EffectRevoke}})); got != totalPerms {
		t.Errorf("EffectiveSet(ren, ...) has %d keys, want %d (pinned)", got, totalPerms)
	}
	// The installed global table is untouched by a pure computation.
	if !Can([]string{"moderator"}, TopicHide) {
		t.Error("EffectiveSet must not mutate the installed resolver")
	}
}

// assertCatalogOrder checks a permission slice follows catalog declaration order.
func assertCatalogOrder(t *testing.T, perms []Permission) {
	t.Helper()
	last := -1
	for _, p := range perms {
		idx := catalogIndex[p]
		if idx <= last {
			t.Errorf("permissions out of catalog order at %q (index %d after %d)", p, idx, last)
		}
		last = idx
	}
}
