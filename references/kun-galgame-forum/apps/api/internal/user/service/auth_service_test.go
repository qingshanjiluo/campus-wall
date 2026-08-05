package service

import (
	"slices"
	"testing"

	"kun-galgame-api/internal/user/oauth"
)

// The login response the FE writes into its store MUST carry the EFFECTIVE role
// set (global ∪ this-site's site_roles), NOT the raw global roles. The original
// integration gap shipped the raw roles here, so a site moderator was enforced
// server-side but the FE store's canModerate stayed false and the moderator UI
// was hidden. Contract docs/oauth/12-site-roles.md §5.1.
func TestNewLoginUserProfileMergesSiteRoles(t *testing.T) {
	u := &oauth.UserInfo{
		ID:        7,
		Sub:       "uuid-7",
		Name:      "kun",
		Roles:     []string{"creator"},   // global
		SiteRoles: []string{"moderator"}, // site-scoped (this site)
	}

	got := newLoginUserProfile(u, "avatar.webp", 42)

	if want := []string{"creator", "moderator"}; !slices.Equal(got.Roles, want) {
		t.Fatalf("login response roles = %v, want the union %v", got.Roles, want)
	}
	if got.ID != 7 || got.Sub != "uuid-7" || got.Name != "kun" ||
		got.Avatar != "avatar.webp" || got.Moemoepoint != 42 {
		t.Fatalf("login profile passthrough wrong: %+v", got)
	}
}

// A plain user (no global role, no site grant) surfaces an empty set — the
// implicit `user` default is never a claim member.
func TestNewLoginUserProfilePlainUser(t *testing.T) {
	got := newLoginUserProfile(&oauth.UserInfo{ID: 1}, "", 0)
	if len(got.Roles) != 0 {
		t.Fatalf("plain user roles = %v, want empty", got.Roles)
	}
}
