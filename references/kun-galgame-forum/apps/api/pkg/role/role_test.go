package role

import (
	"slices"
	"testing"
)

// Golden tests for the site-role union (contract docs/oauth/12-site-roles.md).
// The whole integration is "effective = roles ∪ site_roles, fed to the EXISTING
// capability functions" (§5.1); these lock in that a site grant lands on the
// right capability and, critically, can NEVER reach an admin-only one (§5.3).

// A site moderator (site_roles: ["moderator"], no global roles) gains the
// moderation capability but MUST NOT reach admin — the core safety property.
func TestSiteModeratorModeratesButNotAdminister(t *testing.T) {
	eff := Union(nil, []string{"moderator"})
	if !CanModerate(eff) {
		t.Fatalf("site moderator should CanModerate; effective=%v", eff)
	}
	if CanAdminister(eff) {
		t.Fatalf("site moderator MUST NOT CanAdminister; effective=%v", eff)
	}
	if IsCreator(eff) {
		t.Fatalf("site moderator is not a creator; effective=%v", eff)
	}
}

// A site creator gains only the orthogonal publishing capability.
func TestSiteCreatorIsOrthogonal(t *testing.T) {
	eff := Union([]string{}, []string{"creator"})
	if !IsCreator(eff) {
		t.Fatalf("site creator should IsCreator; effective=%v", eff)
	}
	if CanModerate(eff) || CanAdminister(eff) {
		t.Fatalf("site creator must have no management power; effective=%v", eff)
	}
}

// A site custom bundle name (e.g. event_organizer) fails closed: it maps to no
// built-in capability — the site defines its own meaning (contract §3/§5.4).
func TestSiteCustomNameFailsClosed(t *testing.T) {
	eff := Union(nil, []string{"event_organizer"})
	if CanModerate(eff) || CanAdminister(eff) || IsCreator(eff) {
		t.Fatalf("unknown site role must grant nothing; effective=%v", eff)
	}
}

// Global + site roles combine; the union dedups, is order-stable, and leaves
// the input slices untouched.
func TestUnionDedupAndImmutability(t *testing.T) {
	global := []string{"creator"}
	site := []string{"creator", "moderator"} // 'creator' overlaps global
	eff := Union(global, site)

	if want := []string{"creator", "moderator"}; !slices.Equal(eff, want) {
		t.Fatalf("union = %v, want %v (dedup, order-stable)", eff, want)
	}
	if !IsCreator(eff) || !CanModerate(eff) {
		t.Fatalf("effective should be creator + moderator; effective=%v", eff)
	}
	if CanAdminister(eff) {
		t.Fatalf("no admin power may appear from a site grant; effective=%v", eff)
	}
	if !slices.Equal(global, []string{"creator"}) {
		t.Fatalf("Union mutated its global input: %v", global)
	}
}

// A global admin is unaffected by an empty site set, and Union returns the
// global slice as-is (no allocation churn on the empty-site hot path).
func TestGlobalAdminUnaffectedByEmptySite(t *testing.T) {
	eff := Union([]string{"admin"}, nil)
	if !CanAdminister(eff) || !CanModerate(eff) {
		t.Fatalf("global admin should administer + moderate; effective=%v", eff)
	}
	if !slices.Equal(eff, []string{"admin"}) {
		t.Fatalf("union with empty site = %v, want [admin]", eff)
	}
}
