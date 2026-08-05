package handler

// Route-layer tests for the editing-engine BFF (E3a): the real EditHandler
// over a bare Fiber app wired exactly like router.go (auth stub + moderator
// gates), backed by an httptest fake standing in for the catalog edit face.
// They pin the four things the BFF must guarantee:
//   - an unconfigured catalog client degrades every endpoint to 503 with
//     ZERO S2S traffic (never a local fallback);
//   - the asserted actor shape: roles pass through VERBATIM, trust tier is
//     the conservative staff mapping (staff → 3, everyone else → 0);
//   - local validation rejects foreign field keys before the S2S hop;
//   - reads/writes are pinned to kungal's own tenant (a foreign-site
//     proposal reads as 404) and the moderator entry gates hold.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"kun-galgame-api/internal/galgame/client"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/catalogclient"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

// fakeNotifier records every notification the BFF emits (E3b decision
// notices) without touching a database.
type fakeNotifier struct {
	mu    sync.Mutex
	specs []msgService.Spec
}

func (f *fakeNotifier) Emit(_ *gorm.DB, spec msgService.Spec) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs = append(f.specs, spec)
	return nil
}

func (f *fakeNotifier) EmitMany(tx *gorm.DB, specs []msgService.Spec) error {
	for _, s := range specs {
		_ = f.Emit(tx, s)
	}
	return nil
}

// fakeGalgame serves the ID BRIDGE both directions for one entry: gid 1 ⇄
// registry work 1000. The two are deliberately different numbers — an id that
// happened to match on both sides would hide exactly the bug the bridge exists
// to prevent.
func fakeGalgame(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/catalog/lookup/batch":
			raw, _ := io.ReadAll(r.Body)
			if strings.Contains(string(raw), `"external_id":"1"`) {
				_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[{"external_id":"1","work":{"id":1000}}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[]}}`))
		case "/v1/catalog/works":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[{"id":1000,` +
				`"claimed_by":{"site":"kungal","work_id":1,"state":"live"},` +
				`"names":{"zh-cn":"测试游戏"}}],"next_cursor":null}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":233,"message":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeOwners is the submitter snapshot: entry gid 1 was created by uid 7.
type fakeOwners map[int]int

func (f fakeOwners) OwnerOf(gid int) int { return f[gid] }

// fakeEditFace records every edit-face request and serves canned replies.
type fakeEditFace struct {
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
	// Face is "s2s" for the Basic-authed /api/v1/catalog/edit/* face and "other"
	// for anything else. The BFF speaks exactly one edit channel, so "other" in a
	// recorded request is itself the failure this axis exists to catch.
	Face string
	Auth string // Authorization header (Basic on the S2S face)
}

// s2sFace reports whether a path targets the S2S /api/v1/catalog/edit/* face —
// the only edit channel this BFF has.
func s2sFace(path string) bool { return strings.HasPrefix(path, "/api/v1/catalog/edit/") }

func (f *fakeEditFace) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		face := "other"
		if s2sFace(r.URL.Path) {
			face = "s2s"
		}
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body,
			Face: face, Auth: r.Header.Get("Authorization"),
		})
		f.mu.Unlock()
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/proposals/7/withdraw":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":7,"entity_type":"catalog.work","entity_id":1000,"site":"kungal","status":"withdrawn","patch":{}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/proposals":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"merged":false,"proposal":{"id":7,"entity_type":"catalog.work","entity_id":1000,"site":"kungal","status":"open","patch":{}}}}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/catalog/edit/proposals/7":
			// An open kungal proposal on game 1 by uid 9 — the review target.
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":7,"entity_type":"catalog.work","entity_id":1000,"site":"kungal","status":"open","proposer_uid":9,"patch":{"catalog.work.name_zh_cn":"新标题"}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/proposals/7/merge":
			// The merged revision carries an amender → the notice marks the correction.
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":100,"seq":2,"action":"merged","actor_uid":9,"amender_uid":42,"changed_fields":["catalog.work.name_zh_cn"],"snapshot":{}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/proposals/7/decline":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":7,"entity_type":"catalog.work","entity_id":1000,"site":"kungal","status":"declined","proposer_uid":9,"patch":{}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/proposals/7/amendments":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":1,"seq":1,"amender_uid":7,"set":{"catalog.work.name_zh_cn":"修正"}}}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/catalog/edit/revert":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"proposal":{"id":8,"entity_type":"catalog.work","entity_id":1000,"site":"kungal","status":"merged","patch":{}},"revision":{"id":101,"seq":3,"action":"reverted","actor_uid":7,"changed_fields":["catalog.work.name_zh_cn"],"snapshot":{}}}}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/catalog/edit/proposals/55":
			// A proposal OUTSIDE kungal's tenant — the BFF must 404 it.
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"id":55,"entity_type":"catalog.work","entity_id":1000,"site":"letmoe","status":"open","patch":{}}}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/catalog/edit/snapshot":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"entity_type":"catalog.work","entity_id":1000,"values":{"catalog.work.name_zh_cn":"现值"}}}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/catalog/edit/schema/catalog.work":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"entity_type":"catalog.work","fields":[{"key":"catalog.work.name_zh_cn","kind":"text","diff_hint":"inline","locked":false,"can_propose":true,"can_review":false,"would_automerge":false}]}}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/catalog/edit/proposals":
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":233,"message":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// editTestApp wires the edit routes exactly like router.go, with a stubbed
// session user (nil = anonymous → the auth stub rejects like middleware.Auth).
func editTestApp(t *testing.T, catalogURL string, user *middleware.UserInfo) *fiber.App {
	// The id bridge is wired for every app: it is not optional infrastructure
	// any more, it is how a kungal route reaches an entity at all. The
	// degradation test passes an empty catalog URL, which leaves the EDIT face
	// unconfigured — the thing it is actually asserting.
	return editTestAppFull(t, catalogURL, fakeGalgame(t).URL, user, nil)
}

// editTestAppFull additionally wires a fake galgame (owner lookup / naming) and
// a notifier sink (decision notices). Empty galgameURL / nil notifier = off.
func editTestAppFull(t *testing.T, catalogURL, galgameURL string, user *middleware.UserInfo, notifier msgService.Notifier) *fiber.App {
	t.Helper()
	cc := catalogclient.New(catalogclient.Config{BaseURL: catalogURL, ClientID: "cid", ClientSecret: "sec"})
	var galgameClient *client.GalgameClient
	if galgameURL != "" {
		galgameClient = client.New(galgameURL, "nm_test", "")
	}
	// nil user client / repo = best-effort enrichment off. The submitter lookup
	// is a narrow port so the owner-review gates are testable without a database.
	h := NewEditHandler(cc, galgameClient, nil, notifier, nil).
		WithOwners(fakeOwners{1: 7})

	app := fiber.New()
	authStub := func(c fiber.Ctx) error {
		if user == nil {
			return c.Status(401).JSON(fiber.Map{"code": 205, "message": "用户登录失效"})
		}
		c.Locals(string(middleware.UserInfoKey), user)
		// Mirror middleware.Auth: the session's OAuth access token is exposed for
		// the handlers that forward it (the edit chain does not — it is S2S).
		c.Locals(string(middleware.OAuthAccessTokenKey), "user-jwt")
		return c.Next()
	}
	api := app.Group("/api")
	api.Get("/galgame/:gid/edit/revisions", h.Revisions)
	api.Get("/galgame/:gid/edit/diff", h.Diff)
	api.Get("/galgame/:gid/edit/proposals", h.GameProposals)
	authed := api.Group("", authStub)
	authed.Get("/galgame/:gid/edit/bootstrap", h.Bootstrap)
	authed.Post("/galgame/:gid/edit/proposals", h.Submit)
	authed.Get("/galgame-edit/mine", h.Mine)
	authed.Post("/galgame-edit/proposals/:id/withdraw", h.Withdraw)
	authed.Get("/galgame-edit/queue", middleware.RequireModerator(), h.Queue)
	authed.Get("/galgame-edit/proposals/:id", h.ProposalDetail)
	authed.Post("/galgame-edit/proposals/:id/amend", h.Amend)
	authed.Post("/galgame-edit/proposals/:id/merge", h.Merge)
	authed.Post("/galgame-edit/proposals/:id/decline", h.Decline)
	authed.Post("/galgame/:gid/edit/revert", h.Revert)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path, body string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw
}

var moderatorUser = &middleware.UserInfo{ID: 42, Name: "mod", Roles: []string{"moderator"}}
var adminUser = &middleware.UserInfo{ID: 42, Name: "admin", Roles: []string{"admin"}}
var plainUser = &middleware.UserInfo{ID: 7, Name: "user", Roles: nil}

// TestEditDegradesWhenUnconfigured: every endpoint 503s on an unconfigured
// catalog client, with zero S2S traffic (there is no server to hit).
func TestEditDegradesWhenUnconfigured(t *testing.T) {
	app := editTestApp(t, "", moderatorUser) // empty base URL = not configured
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/galgame/1/edit/bootstrap", ""},
		{"POST", "/api/galgame/1/edit/proposals", `{"patch":{"catalog.work.name_zh_cn":"x"}}`},
		{"GET", "/api/galgame/1/edit/revisions", ""},
		{"GET", "/api/galgame-edit/queue", ""},
		{"GET", "/api/galgame-edit/mine", ""},
	} {
		status, raw := doJSON(t, app, tc.method, tc.path, tc.body)
		if status != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: status = %d body %s, want 503", tc.method, tc.path, status, raw)
		}
	}
}

// TestEditActorAssertionShape pins the S2S actor assertion on submit: the
// proposer's roles pass through verbatim with their trust tier, site is kungal,
// and the dirty-field patch reaches the face untouched. Every proposer — plain
// or staff — rides this one channel.
func TestEditActorAssertionShape(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, moderatorUser)
	status, raw := doJSON(t, app, "POST", "/api/galgame/1/edit/proposals",
		`{"patch":{"catalog.work.name_zh_cn":"新标题"},"note":"typo"}`)
	if status != http.StatusOK {
		t.Fatalf("submit: status = %d body %s", status, raw)
	}
	if len(fake.requests) != 1 || fake.requests[0].Face != "s2s" {
		t.Fatalf("staff submit must be a single S2S call, got %+v", fake.requests)
	}
	body := fake.requests[0].Body
	if body["entity_type"] != "catalog.work" || body["site"] != "kungal" {
		t.Fatalf("entity/site assertion wrong: %v", body)
	}
	patch := body["patch"].(map[string]any)
	if patch["catalog.work.name_zh_cn"] != "新标题" {
		t.Fatalf("patch not passed through verbatim: %v", patch)
	}
	actor := body["actor"].(map[string]any)
	if actor["user_id"] != float64(moderatorUser.ID) {
		t.Fatalf("actor user_id = %v", actor["user_id"])
	}
	if tier, _ := actor["trust_tier"].(float64); tier != 3 {
		t.Fatalf("trust_tier = %v, want 3", actor["trust_tier"])
	}
	roles, _ := actor["roles"].([]any)
	if len(roles) != 1 || roles[0] != "moderator" {
		t.Fatalf("roles = %v, want [moderator]", roles)
	}
}

// TestEditSubmitLocalValidation: foreign field keys and empty patches are
// rejected locally — the S2S face is never touched.
func TestEditSubmitLocalValidation(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, plainUser)
	for _, body := range []string{
		`{"patch":{}}`,
		`{"patch":{"galgame.game.name_zh_cn":"x"}}`,
		`{"patch":{"gid":1}}`,
	} {
		status, _ := doJSON(t, app, "POST", "/api/galgame/1/edit/proposals", body)
		if status != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, status)
		}
	}
	if len(fake.requests) != 0 {
		t.Fatalf("local validation must not reach the S2S face, got %d calls", len(fake.requests))
	}
}

// TestEditReviewEntryGates: the queue 403s for a plain user at the route
// layer with zero S2S traffic; the proposal-directed review surfaces admit
// moderators and the game's creator (E3b) — a plain NON-owner user 403s in
// the handler after the entry check. Bootstrap stays open to any logged-in
// user and anonymous callers are rejected by auth.
func TestEditReviewEntryGates(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, plainUser)
	status, _ := doJSON(t, app, "GET", "/api/galgame-edit/queue", "")
	if status != http.StatusForbidden {
		t.Fatalf("queue as plain user: status = %d, want 403", status)
	}
	if len(fake.requests) != 0 {
		t.Fatalf("the queue gate must not reach the S2S face, got %d calls", len(fake.requests))
	}

	// A plain user who does NOT own game 1 (fake wiki: creator uid 7, this
	// user is 8): every proposal-directed surface 403s after the entry check
	// and no write ever reaches the face.
	nonOwner := &middleware.UserInfo{ID: 8, Name: "bystander", Roles: nil}
	nm := fakeGalgame(t)
	app = editTestAppFull(t, fake.server(t).URL, nm.URL, nonOwner, nil)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/galgame-edit/proposals/7"},
		{"POST", "/api/galgame-edit/proposals/7/amend"},
		{"POST", "/api/galgame-edit/proposals/7/merge"},
		{"POST", "/api/galgame-edit/proposals/7/decline"},
	} {
		status, _ := doJSON(t, app, tc.method, tc.path, `{"note":"x","set":{"catalog.work.name_zh_cn":"y"}}`)
		if status != http.StatusForbidden {
			t.Fatalf("%s %s as non-owner: status = %d, want 403", tc.method, tc.path, status)
		}
	}
	for _, r := range fake.requests {
		if r.Method != "GET" {
			t.Fatalf("non-owner must never reach a write op, got %s %s", r.Method, r.Path)
		}
	}

	anon := editTestApp(t, fake.server(t).URL, nil)
	status, _ = doJSON(t, anon, "GET", "/api/galgame/1/edit/bootstrap", "")
	if status != http.StatusUnauthorized {
		t.Fatalf("anonymous bootstrap: status = %d, want 401", status)
	}
}

// TestEditOwnerReview (E3b): the game's creator — a plain user, no roles —
// passes the review entry, the owner assertion rides the S2S actor, the
// merge lands, and the proposer is notified with the correction marker
// (the merged revision carries an amender).
func TestEditOwnerReview(t *testing.T) {
	fake := &fakeEditFace{}
	nm := fakeGalgame(t)
	sink := &fakeNotifier{}
	app := editTestAppFull(t, fake.server(t).URL, nm.URL, plainUser, sink) // uid 7 = the creator

	status, raw := doJSON(t, app, "GET", "/api/galgame-edit/proposals/7", "")
	if status != http.StatusOK {
		t.Fatalf("owner workbench read: status = %d body %s", status, raw)
	}
	var schemaQuery string
	for _, r := range fake.requests {
		if strings.HasPrefix(r.Path, "/api/v1/catalog/edit/schema/") {
			schemaQuery = r.Query
		}
	}
	if !strings.Contains(schemaQuery, "is_entity_owner=true") {
		t.Fatalf("owner assertion missing from the schema projection query: %q", schemaQuery)
	}

	status, raw = doJSON(t, app, "POST", "/api/galgame-edit/proposals/7/merge", `{"note":""}`)
	if status != http.StatusOK {
		t.Fatalf("owner merge: status = %d body %s", status, raw)
	}
	var mergeBody map[string]any
	for _, r := range fake.requests {
		if r.Method == "POST" && strings.HasSuffix(r.Path, "/merge") {
			mergeBody = r.Body
		}
	}
	actor, _ := mergeBody["actor"].(map[string]any)
	if actor == nil || actor["is_entity_owner"] != true {
		t.Fatalf("owner assertion missing from the merge actor: %v", mergeBody)
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.specs) != 1 {
		t.Fatalf("want exactly one merged notice, got %d", len(sink.specs))
	}
	n := sink.specs[0]
	if n.Kind != msgService.NotifyMerged || n.ReceiverID != 9 || n.SenderID != 7 || n.GalgameID != 1 {
		t.Fatalf("merged notice shape: %+v", n)
	}
	if !strings.Contains(n.Content, "测试游戏") || !strings.Contains(n.Content, "修正") {
		t.Fatalf("merged notice content must name the game + mark the correction: %q", n.Content)
	}
}

// TestEditDeclineNotification: adjudication (decline) requires admin/ren or the
// game's owner — a plain moderator is forbidden (sanctioned split: view is
// moderator+, decide is admin+, mirroring infra edit.catalog.work.review). Once
// a valid decider acts, the decline reason travels to the proposer in full on
// the decline notice (E3b ruling 1).
func TestEditDeclineNotification(t *testing.T) {
	fake := &fakeEditFace{}
	nm := fakeGalgame(t)
	sink := &fakeNotifier{}

	// A plain moderator (not the owner) may VIEW but not DECIDE: decline 403s
	// locally, before any decline write reaches the S2S face.
	modApp := editTestAppFull(t, fake.server(t).URL, nm.URL, moderatorUser, sink)
	if status, raw := doJSON(t, modApp, "POST", "/api/galgame-edit/proposals/7/decline", `{"note":"x"}`); status != http.StatusForbidden {
		t.Fatalf("moderator decline: status = %d body %s, want 403", status, raw)
	}
	for _, r := range fake.requests {
		if r.Method == "POST" && strings.HasSuffix(r.Path, "/decline") {
			t.Fatalf("forbidden moderator decline must not reach the S2S face")
		}
	}

	app := editTestAppFull(t, fake.server(t).URL, nm.URL, adminUser, sink)
	status, raw := doJSON(t, app, "POST", "/api/galgame-edit/proposals/7/decline", `{"note":"资料来源不可靠，请补充出处"}`)
	if status != http.StatusOK {
		t.Fatalf("decline: status = %d body %s", status, raw)
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.specs) != 1 {
		t.Fatalf("want exactly one declined notice, got %d", len(sink.specs))
	}
	n := sink.specs[0]
	if n.Kind != msgService.NotifyDeclined || n.ReceiverID != 9 || n.SenderID != 42 || n.GalgameID != 1 {
		t.Fatalf("declined notice shape: %+v", n)
	}
	if !strings.Contains(n.Content, "资料来源不可靠，请补充出处") {
		t.Fatalf("declined notice must carry the reason in full: %q", n.Content)
	}
}

// TestEditRevertEntry (E3b): revert admits moderators and the game's creator
// — a plain non-owner 403s locally, the owner's revert reaches the S2S face
// with the ownership assertion.
func TestEditRevertEntry(t *testing.T) {
	fake := &fakeEditFace{}
	nm := fakeGalgame(t)

	nonOwner := &middleware.UserInfo{ID: 8, Name: "bystander", Roles: nil}
	app := editTestAppFull(t, fake.server(t).URL, nm.URL, nonOwner, nil)
	status, _ := doJSON(t, app, "POST", "/api/galgame/1/edit/revert", `{"to_seq":3}`)
	if status != http.StatusForbidden {
		t.Fatalf("non-owner revert: status = %d, want 403", status)
	}
	for _, r := range fake.requests {
		if r.Method == "POST" {
			t.Fatalf("non-owner revert must not reach a write op, got %s %s", r.Method, r.Path)
		}
	}

	owner := editTestAppFull(t, fake.server(t).URL, nm.URL, plainUser, nil) // uid 7 = creator
	status, raw := doJSON(t, owner, "POST", "/api/galgame/1/edit/revert", `{"to_seq":3,"note":"回滚测试"}`)
	if status != http.StatusOK {
		t.Fatalf("owner revert: status = %d body %s", status, raw)
	}
	var revertBody map[string]any
	for _, r := range fake.requests {
		if r.Method == "POST" && strings.HasSuffix(r.Path, "/revert") {
			revertBody = r.Body
		}
	}
	if revertBody == nil || revertBody["to_seq"] != float64(3) || revertBody["site"] != "kungal" {
		t.Fatalf("revert body: %v", revertBody)
	}
	actor, _ := revertBody["actor"].(map[string]any)
	if actor == nil || actor["is_entity_owner"] != true {
		t.Fatalf("owner assertion missing from the revert actor: %v", revertBody)
	}
}

// TestEditGameProposals: the per-game proposal list is a public read (the
// old wire's PR list parity).
func TestEditGameProposals(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, nil) // anonymous
	status, raw := doJSON(t, app, "GET", "/api/galgame/1/edit/proposals", "")
	if status != http.StatusOK {
		t.Fatalf("game proposals: status = %d body %s", status, raw)
	}
	if len(fake.requests) != 1 || !strings.Contains(fake.requests[0].Query, "entity_id=1") {
		t.Fatalf("list must filter to the game: %+v", fake.requests)
	}
	if !strings.Contains(fake.requests[0].Query, "status=0") && !strings.Contains(fake.requests[0].Query, "status=open") {
		t.Fatalf("list must default to open proposals: %q", fake.requests[0].Query)
	}
}

// TestEditTenantPin: a proposal on a foreign tenant (site=galgame_wiki, the
// old wire) reads as 404 through this BFF — kungal adjudicates only its own.
func TestEditTenantPin(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, moderatorUser)
	status, raw := doJSON(t, app, "GET", "/api/galgame-edit/proposals/55", "")
	if status != http.StatusNotFound {
		t.Fatalf("foreign-tenant proposal: status = %d body %s, want 404", status, raw)
	}
	status, _ = doJSON(t, app, "POST", "/api/galgame-edit/proposals/55/merge", `{"note":""}`)
	if status != http.StatusNotFound {
		t.Fatalf("foreign-tenant merge: status = %d, want 404", status)
	}
}

// TestEditBootstrapShape: bootstrap bundles the snapshot values + the
// capability projection; can_review reflects the projection.
func TestEditBootstrapShape(t *testing.T) {
	fake := &fakeEditFace{}
	app := editTestApp(t, fake.server(t).URL, plainUser)
	status, raw := doJSON(t, app, "GET", "/api/galgame/1/edit/bootstrap", "")
	if status != http.StatusOK {
		t.Fatalf("bootstrap: status = %d body %s", status, raw)
	}
	var out struct {
		Data struct {
			Gid       int64          `json:"gid"`
			Values    map[string]any `json:"values"`
			Fields    []any          `json:"fields"`
			CanReview bool           `json:"can_review"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Data.Gid != 1 || out.Data.Values["catalog.work.name_zh_cn"] != "现值" {
		t.Fatalf("bootstrap shape wrong: %s", raw)
	}
	if len(out.Data.Fields) != 1 || out.Data.CanReview {
		t.Fatalf("projection shape wrong: %s", raw)
	}
}
