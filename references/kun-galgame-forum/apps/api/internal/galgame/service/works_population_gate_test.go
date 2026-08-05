package service

// The two remaining works-population lanes that reach the catalog search face
// outside the browse/search page itself. Both were found by the R4 sweep and
// both are the same incident class as doc 106 §37: a lane whose population is
// the registry rather than the PUBLISHED registry lists (or offers) works no
// user can open. Neither failure is visible from this side — the multi-tag page
// just shows more cards, the quiz picker just offers more games — so the
// outgoing querystring is pinned here.
//
// The upstream is stubbed empty on purpose: ToCards short-circuits on an empty
// item list and the picker returns before hydrating, so neither needs a
// database or the user service.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"kun-galgame-api/internal/galgame/client"
)

type worksQueryRecorder struct {
	mu    sync.Mutex
	path  string
	query url.Values
}

// client stubs the catalog and returns a galgame client pointed at it.
func (r *worksQueryRecorder) client(t *testing.T) *client.GalgameClient {
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
	return client.New(srv.URL, "nm_test_key", "")
}

func (r *worksQueryRecorder) get(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query.Get(key)
}

// TestMultiTag_AsksForPublishedWorksOnly pins /galgame-tag/multi. It is a
// public 词条 member list like the single-tag /c/{id} page — it just reaches the
// population through the SEARCH face — and it carried no claim gate whatsoever,
// so it listed unclaimed registry rows as well as unpublished claimed ones.
func TestMultiTag_AsksForPublishedWorksOnly(t *testing.T) {
	rec := &worksQueryRecorder{}
	svc := NewTagService(rec.client(t), &GalgameEnricher{}, nil)

	if _, appErr := svc.GetByMultiTag(context.Background(),
		url.Values{"tagIds": {"5,7"}}, true); appErr != nil {
		t.Fatalf("GetByMultiTag: %v", appErr)
	}
	if rec.path != "/v1/catalog/works/search" {
		t.Errorf("path = %q, want /v1/catalog/works/search", rec.path)
	}
	if got := rec.get("claim_state"); got != "live" {
		t.Errorf("claim_state = %q, want live — without it the multi-tag page lists unpublished and unclaimed works", got)
	}
	// The tag scope must still travel, or the page becomes a global browse —
	// and both ids must ride the SAME comma-separated parameter, since the face
	// reads only the first occurrence of a repeated key.
	if got := rec.query["tag_id"]; len(got) != 1 || got[0] != "5,7" {
		t.Errorf("tag_id = %v, want one param valued \"5,7\"", got)
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

// TestQuizPicker_OffersPublishedWorksOnly pins the 出题 galgame picker. This is
// the AUTHORING side of the same leak: `claimed=true` bounds the picker to works
// with a gid, but a claimed-yet-unpublished entry still has one — so a quiz
// could be authored against a game no player can open.
func TestQuizPicker_OffersPublishedWorksOnly(t *testing.T) {
	rec := &worksQueryRecorder{}
	svc := NewQuizService(nil, rec.client(t), nil, nil, nil)

	if got := svc.SearchGalgameOptions(context.Background(), "恋爱", true); len(got) != 0 {
		t.Fatalf("stubbed empty upstream returned %d options", len(got))
	}
	if rec.path != "/v1/catalog/works/search" {
		t.Errorf("path = %q, want /v1/catalog/works/search", rec.path)
	}
	if got := rec.get("claim_state"); got != "live" {
		t.Errorf("claim_state = %q, want live — without it the picker offers unpublished games", got)
	}
	// The face-side claim_state gate is LIVE, so the assertion above pins a
	// filter that really narrows the offer. `claimed=true` rides along as the
	// explicit statement of the other requirement — only a claimed work has a
	// kungal gid to author a quiz against — which a live claim now implies.
	if got := rec.get("claimed"); got != "true" {
		t.Errorf("claimed = %q, want true", got)
	}
}
