package service

// GetByMultiTag builds an upstream query, and every field in it is load-bearing
// in a way that is invisible from this side: a dropped param does not fail, it
// silently returns plausible-but-wrong results. page/limit went unforwarded for
// a long time and the only symptom was that page 2 quietly re-served page 1. So
// the query is pinned here.
//
// The A2-3 re-anchoring changed two things about this lane:
//
//   - the multi-tag AND now rides ONE comma-separated tag_id parameter on the
//     catalog works search, in place of the bespoke wiki endpoint. Repeating
//     the key instead is the one shape that CANNOT work: the upstream reads
//     this filter with Fiber's c.Query, which sees only the first occurrence,
//     so a multi-select degrades to a single-tag filter with no error anywhere;
//   - `mode=contains` no longer expands to descendants. The canonical tag
//     vocabulary has no DAG (refs/proj/126 P2), so there is nothing to expand
//     to, and the mode degrades to exact. The test pins the degradation
//     deliberately: it must be silent-and-correct rather than an error, because
//     the FE persists the mode in saved filters.
//
// The upstream is stubbed with an empty item list on purpose: ToCards
// short-circuits before touching the DB or the user service, so these tests
// need no fixtures and assert exactly one thing — what we asked for.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"kun-galgame-api/internal/galgame/client"
)

// queryRecorder captures the query the catalog face was called with.
type queryRecorder struct {
	mu    sync.Mutex
	path  string
	query url.Values
}

func (r *queryRecorder) service(t *testing.T) *TagService {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.path = req.URL.Path
		r.query = req.URL.Query()
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[],"total":0,"page":1,"limit":20}}`))
	}))
	t.Cleanup(srv.Close)
	return NewTagService(client.New(srv.URL, "nm_test_key", ""), &GalgameEnricher{}, nil)
}

func (r *queryRecorder) get(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query.Get(key)
}

func (r *queryRecorder) all(key string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.query[key]
}

func (r *queryRecorder) has(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.query[key]
	return ok
}

func (r *queryRecorder) urlPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path
}

func TestGetByMultiTag_ForwardsPagination(t *testing.T) {
	rec := &queryRecorder{}
	svc := rec.service(t)

	_, appErr := svc.GetByMultiTag(context.Background(), url.Values{
		"tagIds": {"638,41"},
		"page":   {"3"},
		"limit":  {"24"},
	}, false)
	if appErr != nil {
		t.Fatalf("GetByMultiTag: %v", appErr)
	}

	// The search lane is the one that carries page/limit + total; the browse
	// lane is keyset-only and could not answer this view.
	if got := rec.urlPath(); got != "/v1/catalog/works/search" {
		t.Errorf("path = %q, want /v1/catalog/works/search", got)
	}
	// Flat AND = ONE tag_id parameter carrying every id, comma-separated.
	if got := rec.all("tag_id"); len(got) != 1 || got[0] != "638,41" {
		t.Errorf("tag_id = %v, want exactly one param valued \"638,41\"", got)
	}
	// The regression this pins: without these the face answers with its own
	// default first page for every request, so the pager moves and nothing changes.
	if got := rec.get("page"); got != "3" {
		t.Errorf("page = %q, want %q — pagination must reach upstream", got, "3")
	}
	if got := rec.get("limit"); got != "24" {
		t.Errorf("limit = %q, want %q", got, "24")
	}
	if got := rec.get("include"); got != CatalogCardInclude {
		t.Errorf("include = %q, want %q (cards render names + covers)", got, CatalogCardInclude)
	}
	// NSFW must travel as a request parameter — post-filtering downstream is
	// forbidden (the data has already left the boundary by then).
	if got := rec.get("nsfw"); got != "1" {
		t.Errorf("nsfw = %q, want 1 on every lane", got)
	}
	if got := rec.get("content_limit"); got != "" {
		t.Errorf("content_limit = %q, want it absent — an NSFW caller opts out of the editorial gate", got)
	}
}

func TestGetByMultiTag_SFWGateStaysClosed(t *testing.T) {
	rec := &queryRecorder{}
	svc := rec.service(t)

	if _, appErr := svc.GetByMultiTag(context.Background(),
		url.Values{"tagIds": {"41"}}, true); appErr != nil {
		t.Fatalf("GetByMultiTag: %v", appErr)
	}
	// SFW is the EDITORIAL gate now: an SFW caller sees every r18 game whose
	// kungal entry an editor graded sfw, which is 94.5% of the catalogue.
	if got := rec.get("nsfw"); got != "1" {
		t.Errorf("nsfw = %q, want 1 — the age gate is never a population cut", got)
	}
	if got := rec.get("content_limit"); got != "sfw" {
		t.Errorf("content_limit = %q, want sfw for an SFW caller", got)
	}
}

// TestGetByMultiTag_TagFilterShape pins the WIRE SHAPE of the tag filter, which
// is the only thing that distinguishes a working multi-select from a broken one
// here: a repeated tag_id is a perfectly valid querystring, the catalog answers
// 200 and the page renders — with every tag after the first silently dropped,
// because that face reads the filter with Fiber's c.Query (first occurrence
// only). Exactly one key, comma-joined value, capped at the contract's ten.
func TestGetByMultiTag_TagFilterShape(t *testing.T) {
	for name, tc := range map[string]struct {
		tagIDs string
		want   []string
	}{
		"one id travels alone":        {"41", []string{"41"}},
		"a multi-select is ONE param": {"638,41,7", []string{"638,41,7"}},
		// Past ten ids the contract answers 400, so the selection is truncated
		// rather than turned into a failed view.
		"over the cap is truncated to ten": {
			"1,2,3,4,5,6,7,8,9,10,11,12",
			[]string{"1,2,3,4,5,6,7,8,9,10"},
		},
		"no selection sends no filter at all": {"", nil},
	} {
		t.Run(name, func(t *testing.T) {
			rec := &queryRecorder{}
			svc := rec.service(t)

			if _, appErr := svc.GetByMultiTag(context.Background(),
				url.Values{"tagIds": {tc.tagIDs}}, false); appErr != nil {
				t.Fatalf("GetByMultiTag: %v", appErr)
			}

			got := rec.all("tag_id")
			if len(got) != len(tc.want) {
				t.Fatalf("tag_id = %v (%d params), want %v", got, len(got), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("tag_id[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestGetByMultiTag_MatchMode(t *testing.T) {
	// Every mode now produces the SAME flat AND query: the descendant expansion
	// died with the wiki tag DAG (P2). What must NOT happen is the forum
	// inventing a substring approximation — that expands 恋爱 into 无恋爱剧情,
	// the literal opposite, because name similarity is not a semantic relation.
	for _, mode := range []string{"contains", "exact", "", "fuzzy"} {
		t.Run("mode="+mode, func(t *testing.T) {
			rec := &queryRecorder{}
			svc := rec.service(t)

			q := url.Values{"tagIds": {"41"}}
			if mode != "" {
				q.Set("mode", mode)
			}
			if _, appErr := svc.GetByMultiTag(context.Background(), q, false); appErr != nil {
				t.Fatalf("GetByMultiTag: %v", appErr)
			}

			if got := rec.all("tag_id"); len(got) != 1 || got[0] != "41" {
				t.Errorf("tag_id = %v, want [41] regardless of mode", got)
			}
			// The retired expansion parameter must not be resurrected.
			if rec.has("expand") {
				t.Error("expand leaked upstream; the canonical vocabulary has no DAG to expand into")
			}
			// `mode` is the forum's own vocabulary; upstream never sees it.
			if rec.has("mode") {
				t.Error("mode leaked upstream")
			}
		})
	}
}
