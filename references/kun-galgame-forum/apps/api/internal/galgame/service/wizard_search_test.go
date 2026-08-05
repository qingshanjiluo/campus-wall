package service

// The publish wizard is the site's only defence against duplicate submissions,
// so both halves of its query are pinned here:
//
//   - the ITEMS half must hit the catalog search with claim_state=live,draft,
//     pending. Narrowing that to `live` hides every unpublished entry, which is
//     the shape of the 52k incident: what the wizard cannot see gets submitted
//     again.
//   - the PENDING half must hit the registry's PER-USER claim face. It used to
//     ride the wiki search's include_pending merge, which is why it could only
//     be asked as part of a search; it is now a question in its own right.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/pkg/catalogclient"
)

type wizardRecorder struct {
	mu        sync.Mutex
	catalogQ  url.Values
	claimsQ   url.Values
	claimsHit int
	wikiHits  int
}

func (r *wizardRecorder) service(t *testing.T) *SubmissionService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		body := `{"code":0,"message":"ok","data":{"items":[],"total":0}}`
		switch {
		case strings.HasSuffix(req.URL.Path, "/catalog/works/search"):
			r.catalogQ = req.URL.Query()
			body = `{"code":0,"message":"ok","data":{"total":2,"items":[
			  {"id":11,"display_name":"A","cover":"https://img/aa/bb/hash1.webp",
			   "claimed_by":{"site":"kungal","work_id":292,"state":"live"},
			   "names":{"ja-jp":"白恋サクラ"},"refs":[{"source":"vndb","external_id":"v22610"}]},
			  {"id":12,"display_name":"B","cover":"",
			   "claimed_by":{"site":"kungal","work_id":9978,"state":"draft"}},
			  {"id":13,"display_name":"withdrawn","cover":"",
			   "claimed_by":{"site":"kungal","work_id":404,"state":"hidden"}},
			  {"id":14,"display_name":"unclaimed","cover":"","claimed_by":null}
			]}}`
		case strings.Contains(req.URL.Path, "/claims"):
			r.claimsQ = req.URL.Query()
			r.claimsHit++
			body = `{"code":0,"message":"ok","data":{"items":[
			  {"work_id":64689,"display_name":"曇った瞳に恋してる","site":"kungal",
			   "product_work_id":64689,"claim_state":"pending","last_event_id":9,
			   "last_from_state":"draft","last_to_state":"pending","last_reason":null,
			   "last_actor_uid":7,"last_event_at":"2026-07-31T00:00:00Z",
			   "first_acted_at":"2026-07-31T00:00:00Z","acted_count":1}
			],"next_before":0,"total":1}}`
		case strings.HasSuffix(req.URL.Path, "/galgame/search"):
			r.wikiHits++
		}
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewSubmissionService(
		client.New(srv.URL, "nm_test_key", ""),
		catalogclient.New(catalogclient.Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"}),
		nil,
	)
}

func wizardSearch(t *testing.T, svc *SubmissionService) *WizardSearchPage {
	t.Helper()
	page, appErr := svc.SearchWithPending(t.Context(), 7,
		url.Values{"q": {"sakura"}, "limit": {"12"}})
	if appErr != nil {
		t.Fatalf("SearchWithPending: %v", appErr)
	}
	return page
}

func TestWizard_ItemsComeFromTheCatalogSearch(t *testing.T) {
	rec := &wizardRecorder{}
	page := wizardSearch(t, rec.service(t))

	// `pending` is asked for BEFORE the projector produces it, so the wizard and
	// the projector fix need not ship together. Dropping it here would mean a
	// second coordinated deploy the day the projector starts separating "nobody
	// has claimed this" from "somebody is waiting on a review".
	if got := rec.catalogQ.Get("claim_state"); got != "live,draft,pending" {
		t.Errorf("claim_state = %q, want live,draft,pending — `live` alone hides every unpublished entry", got)
	}
	if got := rec.catalogQ.Get("claimed"); got != "true" {
		t.Errorf("claimed = %q, want true — an unclaimed work has no gid to act on", got)
	}
	if got := rec.catalogQ.Get("q"); got != "sakura" {
		t.Errorf("q = %q, want sakura", got)
	}
	if got := rec.catalogQ.Get("limit"); got != "12" {
		t.Errorf("limit = %q, want 12", got)
	}
	// The age gate is open and the EDITORIAL gate is absent: the wizard is a
	// dedup tool for a submitter, not a browse lane.
	if got := rec.catalogQ.Get("nsfw"); got != "1" {
		t.Errorf("nsfw = %q, want 1", got)
	}
	if got := rec.catalogQ.Get("content_limit"); got != "" {
		t.Errorf("content_limit = %q, want it absent on the wizard lane", got)
	}
	if !strings.Contains(rec.catalogQ.Get("include"), "refs") {
		t.Errorf("include = %q, want refs (the row prints the VNDB id)", rec.catalogQ.Get("include"))
	}
	if page.Total != 2 {
		t.Errorf("total = %d, want the catalog total 2", page.Total)
	}
}

func TestWizard_ItemsAreKeyedByGIDAndDropWithdrawnRows(t *testing.T) {
	rec := &wizardRecorder{}
	page := wizardSearch(t, rec.service(t))

	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2 (hidden claim and unclaimed row are not actionable)", len(page.Items))
	}
	// Never the catalog id: the two key spaces overlap and every wizard action
	// (claim POST, /galgame/:gid link, draft link) is keyed by gid.
	if page.Items[0].ID != 292 || page.Items[1].ID != 9978 {
		t.Errorf("ids = %d,%d, want the gids 292,9978", page.Items[0].ID, page.Items[1].ID)
	}
	if page.Items[0].VndbID != "v22610" {
		t.Errorf("vndb_id = %q, want v22610", page.Items[0].VndbID)
	}
	// The card reads `banner`; the catalog delivers the art as the derived
	// effective banner, so the field must be filled from it or every row on the
	// wizard loses its cover.
	if page.Items[0].Banner == "" || page.Items[0].Banner != page.Items[0].EffectiveBannerURL {
		t.Errorf("banner = %q, want it mirrored from effective_banner_url %q",
			page.Items[0].Banner, page.Items[0].EffectiveBannerURL)
	}
}

func TestWizard_PendingComesFromThePerUserClaimFace(t *testing.T) {
	rec := &wizardRecorder{}
	page := wizardSearch(t, rec.service(t))

	if rec.claimsHit != 1 {
		t.Fatalf("per-user claim face hits = %d, want exactly 1", rec.claimsHit)
	}
	if rec.wikiHits != 0 {
		t.Errorf("wiki face hits = %d, want 0 — the pending half is terminal now", rec.wikiHits)
	}
	// Scoped to kungal's own tenant and to the states a submitter is waiting on:
	// an unscoped query would show another product's backlog, and including
	// `live` would put finished entries back into "还没通过".
	if got := rec.claimsQ.Get("site"); got != "kungal" {
		t.Errorf("site = %q, want kungal", got)
	}
	if got := rec.claimsQ.Get("claim_state"); got != "pending,declined" {
		t.Errorf("claim_state = %q, want pending,declined", got)
	}
	if len(page.Pending) != 1 || page.Pending[0].WorkID != 64689 {
		t.Fatalf("pending = %+v, want the caller's own pending claim", page.Pending)
	}
	// The row carries the LATEST transition by anyone — which is how a decline
	// reason reaches the submitter without a second query.
	if page.Pending[0].ClaimState != "pending" || page.Pending[0].LastToState != "pending" {
		t.Errorf("pending row = %+v, want the current state and its last transition", page.Pending[0])
	}
}

func TestWizard_PendingIsAnEmptyArrayWhenTheUserHasNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":null,"next_before":0,"total":0}}`))
	}))
	t.Cleanup(srv.Close)
	svc := NewSubmissionService(
		client.New(srv.URL, "nm_test_key", ""),
		catalogclient.New(catalogclient.Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"}),
		nil,
	)

	page := wizardSearch(t, svc)
	// `null` would make the FE's `pending.length` read throw on some paths; an
	// empty array is the shape the component already handles.
	if page.Pending == nil || len(page.Pending) != 0 {
		t.Errorf("pending = %v, want an empty array", page.Pending)
	}
}
