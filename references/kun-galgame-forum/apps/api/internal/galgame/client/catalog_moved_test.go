package client

// Merged-label contract (doc 148). The catalog answers a detail request for an
// id that was merged away with 301 + {code: 12, data: {current_id}}, so the
// 会社 page can hop to the survivor in ONE redirect instead of rendering the
// ghost the old behaviour produced (200, the dead label's name, an empty
// catalogue — a real-looking but permanently empty company page).
//
// The sharp edge these tests guard is the HTTP client itself: net/http follows
// a 301 by default, which would swallow the signal and hand back the SURVIVOR's
// record under the DEAD id — one entity on two URLs, the exact outcome the
// redirect exists to prevent. Hermetic: an httptest server plays the catalog.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// movedCatalog plays a catalog holding one live label (survivor) whose two
// merged predecessors both answer 301. It records every path it is asked for,
// which is how the "did the client follow the redirect" assertion is made.
func movedCatalog(t *testing.T, survivor string) (*GalgameClient, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/catalog/labels/" + survivor:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"id":6935,"display_name":"生存ブランド","kind":"game_brand","work_count":12}}`))
		case "/v1/catalog/labels/13323", "/v1/catalog/labels/13324":
			w.Header().Set("Location", "/v1/catalog/labels/"+survivor)
			w.WriteHeader(http.StatusMovedPermanently)
			_, _ = w.Write([]byte(`{"code":12,"message":"this id was merged away; use current_id",` +
				`"data":{"entity_type":"label","id":13323,"current_id":6935}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":4,"message":"资源不存在"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-key", ""), &seen
}

func TestCatalogLabelMergedIDReportsSurvivor(t *testing.T) {
	c, seen := movedCatalog(t, "6935")

	rec, found, movedTo, appErr := c.CatalogLabel(context.Background(), "13323")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if found {
		t.Fatal("a merged id must not be reported as found — the caller would render the ghost page")
	}
	if movedTo != 6935 {
		t.Fatalf("movedTo = %d, want 6935", movedTo)
	}
	if rec.DisplayName != "" {
		t.Fatalf("the survivor's content must never travel under the dead id, got %q", rec.DisplayName)
	}
	// One request, to the dead id only: following the redirect would show the
	// survivor's path here too.
	if len(*seen) != 1 || (*seen)[0] != "/v1/catalog/labels/13323" {
		t.Fatalf("client followed the redirect: %v", *seen)
	}
}

func TestCatalogLabelLiveAndAbsentUnaffected(t *testing.T) {
	c, _ := movedCatalog(t, "6935")

	rec, found, movedTo, appErr := c.CatalogLabel(context.Background(), "6935")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if !found || movedTo != 0 || rec.DisplayName != "生存ブランド" {
		t.Fatalf("live label mangled: found=%v movedTo=%d name=%q", found, movedTo, rec.DisplayName)
	}

	_, found, movedTo, appErr = c.CatalogLabel(context.Background(), "999999")
	if appErr != nil {
		t.Fatalf("an absent id is a miss, not an error: %v", appErr)
	}
	if found || movedTo != 0 {
		t.Fatalf("absent id: found=%v movedTo=%d, want false/0", found, movedTo)
	}
}

// A chained merge is flattened upstream, so the client sees ONE 301 whose
// current_id is already final — it must never chase a second hop of its own.
func TestCatalogLabelChainIsResolvedUpstream(t *testing.T) {
	c, seen := movedCatalog(t, "6935")

	_, _, movedTo, appErr := c.CatalogLabel(context.Background(), "13324")
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if movedTo != 6935 {
		t.Fatalf("movedTo = %d, want the final survivor 6935", movedTo)
	}
	if len(*seen) != 1 {
		t.Fatalf("expected exactly one upstream call, got %v", *seen)
	}
}
