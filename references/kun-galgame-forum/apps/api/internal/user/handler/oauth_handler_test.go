package handler

import (
	"slices"
	"testing"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/user/dto"
)

// /auth/me is the FE's sole roles-refresh source. Its profile is built from the
// userclient brief, which carries only GLOBAL roles; the handler MUST overlay
// the session's EFFECTIVE roles (global ∪ site_roles) or a site moderator's
// canModerate stays false in the FE. Contract docs/oauth/12-site-roles.md §5.1.
func TestEnrichProfileFromSessionSurfacesEffectiveRoles(t *testing.T) {
	// Brief-derived profile: global roles only (a site grant isn't visible here).
	p := &dto.UserProfile{ID: 7, Name: "kun", Roles: []string{"creator"}}
	// Session: the effective set, already merged at ingestion.
	u := &middleware.UserInfo{Sub: "uuid-7", Roles: []string{"creator", "moderator"}}

	enrichProfileFromSession(p, u)

	if want := []string{"creator", "moderator"}; !slices.Equal(p.Roles, want) {
		t.Fatalf("/auth/me roles = %v, want the session's effective set %v", p.Roles, want)
	}
	if p.Sub != "uuid-7" {
		t.Fatalf("Sub not filled from session: %q", p.Sub)
	}
}
