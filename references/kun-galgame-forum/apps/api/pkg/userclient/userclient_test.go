package userclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// The /users/batch brief is site-scoped (contract docs/oauth/12-site-roles.md
// §2.3), so the client folds this-site's site_roles into Roles — every consumer
// (profile badge, purge protection, isCreator) then reads ONE effective set.
// Guards that a site moderator surfaces as a moderator for OTHER users.
func TestUserBriefFoldsSiteRolesIntoRoles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// One user: global creator + a site-scoped moderator grant on our site.
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"users":[` +
			`{"id":7,"uuid":"u-7","name":"kun","roles":["creator"],"site_roles":["moderator"]}` +
			`],"not_found":[]}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})

	u, ok, err := c.User(context.Background(), 7)
	if err != nil || !ok {
		t.Fatalf("User() ok=%v err=%v", ok, err)
	}
	if want := []string{"creator", "moderator"}; !slices.Equal(u.Roles, want) {
		t.Fatalf("brief Roles = %v, want the effective union %v", u.Roles, want)
	}
	// Raw site claim is retained so a future site-vs-global distinction can use it.
	if want := []string{"moderator"}; !slices.Equal(u.SiteRoles, want) {
		t.Fatalf("brief SiteRoles = %v, want %v", u.SiteRoles, want)
	}
}

// A user with no site grant is unchanged (global roles only).
func TestUserBriefNoSiteRoles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"","data":{"users":[` +
			`{"id":8,"uuid":"u-8","name":"ren","roles":["admin"]}` +
			`],"not_found":[]}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "x", ClientSecret: "y"})
	u, _, _ := c.User(context.Background(), 8)
	if want := []string{"admin"}; !slices.Equal(u.Roles, want) {
		t.Fatalf("brief Roles = %v, want %v (no site grant → unchanged)", u.Roles, want)
	}
}
