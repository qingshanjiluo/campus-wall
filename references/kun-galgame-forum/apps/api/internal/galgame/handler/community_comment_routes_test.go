package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-api/internal/galgame/handler"

	"github.com/gofiber/fiber/v3"
)

// routeExists reports whether the app has a route with the given method + path.
func routeExists(app *fiber.App, method, path string) bool {
	for _, r := range app.GetRoutes() {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

var communityRoutes = []struct{ method, path string }{
	{http.MethodGet, "/galgame/:gid/comments"},
	{http.MethodGet, "/galgame/:gid/comments/locate"},
	{http.MethodPost, "/galgame/:gid/comments"},
	{http.MethodPut, "/galgame/comments/:postId"},
	{http.MethodDelete, "/galgame/comments/:postId"},
	{http.MethodPut, "/galgame/comments/:postId/like"},
	{http.MethodPost, "/galgame/comments/:postId/flag"},
}

// TestCommunityRoutesMountedUnconditionally proves all seven comment routes are
// registered. Community is the unconditional comment backend since the legacy
// /comment routes were retired (charter step 06a) — there is no longer a flag.
func TestCommunityRoutesMountedUnconditionally(t *testing.T) {
	app := fiber.New()
	h := handler.NewCommunityCommentHandler(nil)
	h.RegisterReads(app)
	h.RegisterWrites(app)

	for _, rt := range communityRoutes {
		if !routeExists(app, rt.method, rt.path) {
			t.Errorf("route %s %s NOT registered", rt.method, rt.path)
		}
	}
}

// TestCommunityReadsAnonymous pins the router-ordering regression that made the
// public reads login-only: mounted like the real router (reads BEFORE a
// mandatory-auth Use, writes after), an anonymous GET must reach the read
// handler instead of being rejected by the auth middleware.
func TestCommunityReadsAnonymous(t *testing.T) {
	app := fiber.New()
	h := handler.NewCommunityCommentHandler(nil)

	hit := false
	app.Get("/probe-read-side/:gid/comments", func(c fiber.Ctx) error {
		hit = true
		return c.SendStatus(http.StatusOK)
	})
	h.RegisterReads(app)
	// The mandatory-auth boundary, as in router.go — everything after is gated.
	app.Use(func(c fiber.Ctx) error { return c.SendStatus(http.StatusUnauthorized) })
	h.RegisterWrites(app)

	// The probe route proves stack position semantics hold in this harness.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/probe-read-side/1/comments", nil))
	if err != nil {
		t.Fatalf("app.Test probe: %v", err)
	}
	if resp.StatusCode != http.StatusOK || !hit {
		t.Fatalf("probe route = %d (hit=%v), harness broken", resp.StatusCode, hit)
	}

	// Anonymous read reaches the handler (nil service -> it may 4xx/5xx on its
	// own, but it must NOT be the boundary's 401).
	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/galgame/abc/comments", nil))
	if err != nil {
		t.Fatalf("app.Test read: %v", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("anonymous GET /galgame/:gid/comments was rejected by the mandatory-auth boundary — reads mounted on the wrong side")
	}

	// Writes stay behind the boundary: anonymous POST is rejected by it.
	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/galgame/1/comments", nil))
	if err != nil {
		t.Fatalf("app.Test write: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous POST = %d, want the boundary's 401", resp.StatusCode)
	}
}
