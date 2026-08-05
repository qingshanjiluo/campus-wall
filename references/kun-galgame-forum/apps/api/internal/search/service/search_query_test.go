package service

// The galgame search lane's upstream query is pinned here because the failure
// it guards against is silent: drop `claim_state=live` and the result set
// quietly WIDENS to every registry row — unclaimed VNDB stubs and withdrawn
// entries included — which is the production incident of 2026-07-31 (doc 106
// §37 revoked the whole-registry population A2-3 had opened).
//
// The upstream is stubbed empty on purpose: ToCards short-circuits on an empty
// item list, so this needs neither a database nor the user service.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"kun-galgame-api/internal/galgame/client"
	galgameService "kun-galgame-api/internal/galgame/service"
)

type searchRecorder struct {
	mu    sync.Mutex
	path  string
	query url.Values
}

func (r *searchRecorder) service(t *testing.T) *SearchService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.path = req.URL.Path
		r.query = req.URL.Query()
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[],"total":0}}`))
	}))
	t.Cleanup(srv.Close)
	return NewSearchService(nil, client.New(srv.URL, "nm_test_key", ""), &galgameService.GalgameEnricher{}, nil)
}

func (r *searchRecorder) get(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query.Get(key)
}

func TestSearchGalgames_AsksForPublishedWorksOnly(t *testing.T) {
	rec := &searchRecorder{}
	svc := rec.service(t)

	if _, appErr := svc.SearchGalgames(context.Background(), "恋爱", 1, 24, true); appErr != nil {
		t.Fatalf("SearchGalgames: %v", appErr)
	}
	if rec.path != "/v1/catalog/works/search" {
		t.Errorf("path = %q, want /v1/catalog/works/search", rec.path)
	}
	// The gate itself. `live` is the exact successor of the deprecated face's
	// status=0; anything else (including an absent parameter) puts unpublished
	// works back in front of users.
	if got := rec.get("claim_state"); got != "live" {
		t.Errorf("claim_state = %q, want live — without it search leaks unpublished works", got)
	}
	// The gate is a REQUEST parameter, never a post-filter, so the total the
	// pager reads is gated by the same expression as the items.
	if got := rec.get("q"); got != "恋爱" {
		t.Errorf("q = %q, want the raw keywords", got)
	}
	if got := rec.get("sort"); got != "relevance" {
		t.Errorf("sort = %q, want relevance", got)
	}
	// The SFW preference travels as the EDITORIAL gate; the age gate stays open
	// on every lane, because 94.5% of the registry is r18 and closing it deletes
	// the catalogue instead of filtering adult presentation (doc 106 §38).
	if got := rec.get("nsfw"); got != "1" {
		t.Errorf("nsfw = %q, want 1 — the age gate is never a population cut", got)
	}
	if got := rec.get("content_limit"); got != "sfw" {
		t.Errorf("content_limit = %q, want sfw for an SFW caller", got)
	}
}

func TestSearchGalgames_NSFWCallerStillOnlySeesPublished(t *testing.T) {
	rec := &searchRecorder{}
	svc := rec.service(t)

	if _, appErr := svc.SearchGalgames(context.Background(), "恋爱", 1, 24, false); appErr != nil {
		t.Fatalf("SearchGalgames: %v", appErr)
	}
	if got := rec.get("nsfw"); got != "1" {
		t.Errorf("nsfw = %q, want 1 for an NSFW-opted caller", got)
	}
	if got := rec.get("content_limit"); got != "" {
		t.Errorf("content_limit = %q, want it absent — an NSFW caller opts out of the editorial gate", got)
	}
	// The two gates are independent: opting into r18 must not open the
	// lifecycle gate as a side effect.
	if got := rec.get("claim_state"); got != "live" {
		t.Errorf("claim_state = %q, want live for an NSFW caller too", got)
	}
}
