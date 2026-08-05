package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"kun-galgame-api/internal/galgame/handler"

	"github.com/gofiber/fiber/v3"
)

var resourceCommentRoutes = []struct{ method, path string }{
	{http.MethodGet, "/galgame-rating/:id/comments"},
	{http.MethodGet, "/website/:domain/comments"},
	{http.MethodGet, "/toolset/:id/comments"},
	{http.MethodGet, "/galgame-resource/:id/comments"},
	{http.MethodGet, "/galgame-quiz/:id/comments"},
	{http.MethodPost, "/galgame-rating/:id/comments"},
	{http.MethodDelete, "/galgame-rating/:id/comments/:postId"},
	{http.MethodPost, "/website/:domain/comments"},
	{http.MethodDelete, "/website/:domain/comments/:postId"},
	{http.MethodPost, "/toolset/:id/comments"},
	{http.MethodDelete, "/toolset/:id/comments/:postId"},
	{http.MethodPost, "/galgame-resource/:id/comments"},
	{http.MethodDelete, "/galgame-resource/:id/comments/:postId"},
	{http.MethodPost, "/galgame-quiz/:id/comments"},
	{http.MethodDelete, "/galgame-quiz/:id/comments/:postId"},
}

// TestResourceCommentRoutesMountedUnconditionally proves all fifteen resource
// comment routes are registered. Community is the unconditional comment backend
// since the legacy singular /comment routes were retired (charter step 06a) —
// there is no longer a flag.
func TestResourceCommentRoutesMountedUnconditionally(t *testing.T) {
	app := fiber.New()
	h := handler.NewResourceCommentHandler(nil)
	h.RegisterReads(app)
	h.RegisterWrites(app)

	for _, rt := range resourceCommentRoutes {
		if !routeExists(app, rt.method, rt.path) {
			t.Errorf("route %s %s NOT registered", rt.method, rt.path)
		}
	}
}

// TestResourceCommentReadsAnonymous pins the router-ordering rule that keeps the
// public reads anonymous: mounted like the real router (reads BEFORE a
// mandatory-auth Use, writes after), an anonymous GET must reach the read handler
// instead of being rejected by the auth middleware; an anonymous POST must be
// rejected by the boundary.
func TestResourceCommentReadsAnonymous(t *testing.T) {
	app := fiber.New()
	h := handler.NewResourceCommentHandler(nil)

	h.RegisterReads(app)
	// The mandatory-auth boundary, as in router.go — everything after is gated.
	app.Use(func(c fiber.Ctx) error { return c.SendStatus(http.StatusUnauthorized) })
	h.RegisterWrites(app)

	// Anonymous read reaches the handler: a non-numeric id fails parsing and 400s
	// (proving the handler ran) rather than being intercepted as the boundary 401.
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/galgame-rating/abc/comments", nil))
	if err != nil {
		t.Fatalf("app.Test read: %v", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("anonymous GET /galgame-rating/:id/comments was rejected by the mandatory-auth boundary — reads mounted on the wrong side")
	}

	// Writes stay behind the boundary: anonymous POST is rejected by it.
	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/toolset/1/comments", nil))
	if err != nil {
		t.Fatalf("app.Test write: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous POST = %d, want the boundary's 401", resp.StatusCode)
	}
}

// TestResourceCommentReadsDoNotCollideWithDetailRoutes pins the router-ordering
// claim for the two areas whose comment path hangs off an existing 2-segment
// detail read: /galgame-resource/:id and /galgame-quiz/:id are registered FIRST in
// router.go, and the 3-segment comment reads come later via RegisterReads. That is
// only safe because a Fiber `:param` never spans a `/`. Registered in the real
// (detail-first) order, a 3-segment request must still reach the comment handler.
func TestResourceCommentReadsDoNotCollideWithDetailRoutes(t *testing.T) {
	app := fiber.New()

	// The pre-existing 2-segment detail reads, registered first as in router.go.
	detailHit := false
	detail := func(c fiber.Ctx) error {
		detailHit = true
		return c.SendStatus(http.StatusOK)
	}
	app.Get("/galgame-resource/:id", detail)
	app.Get("/galgame-quiz/:id", detail)

	handler.NewResourceCommentHandler(nil).RegisterReads(app)

	// A non-numeric id makes the comment handler 400 on parse — proof it ran, with
	// no service call. The detail handler must not have been reached.
	for _, path := range []string{"/galgame-resource/abc/comments", "/galgame-quiz/abc/comments"} {
		detailHit = false
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("app.Test %s: %v", path, err)
		}
		if detailHit {
			t.Errorf("GET %s was swallowed by the 2-segment detail route", path)
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400 from the comment handler's id parse", path, resp.StatusCode)
		}
	}
}
