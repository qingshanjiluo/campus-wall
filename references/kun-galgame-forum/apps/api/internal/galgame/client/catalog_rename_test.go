package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// Two identity keys move during the W1 window — the source-12 key that anchors
// gid lookups (galgame_wiki -> curated) and the claim site (galgame_wiki ->
// kungal). Both are read here, both are written elsewhere, and both fail
// SILENTLY when reader and data disagree: an unmatched source resolves no gid
// at all, and an unmatched site yields gid 0, which strips a card's link and
// its local stats without raising anything.
//
// So the reader accepts both spellings rather than being flipped in lockstep
// with the data. These tests pin that tolerance in both directions; when the
// legacy halves are eventually removed, they are what should fail first.

func TestClaimSiteAcceptedOnBothSpellings(t *testing.T) {
	cases := []struct {
		site string
		want int
	}{
		{"kungal", 4321},
		{"galgame_wiki", 4321},
		{"moyu", 0},
		{"", 0},
	}
	for _, tc := range cases {
		t.Run(tc.site, func(t *testing.T) {
			it := &CatalogWorkListItem{ClaimedBy: &catClaimedBy{Site: tc.site, WorkID: 4321}}
			if got := it.gid(); got != tc.want {
				t.Errorf("gid() on site %q = %d, want %d", tc.site, got, tc.want)
			}
		})
	}
	if (&CatalogWorkListItem{}).gid() != 0 {
		t.Error("an unclaimed row has no gid")
	}
}

// The batch lookup must ask for every source key in flight, so it resolves on
// either side of the rename without a coordinated deploy.
func TestAnchorLookupAsksForEverySourceKey(t *testing.T) {
	// Only the LEGACY key answers, i.e. the state before the infra rename.
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Items []struct {
				Source     string `json:"source"`
				ExternalID string `json:"external_id"`
			} `json:"items"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		out := make([]string, 0, len(body.Items))
		for _, it := range body.Items {
			asked = append(asked, it.Source)
			work := "null"
			if it.Source == "galgame_wiki" && it.ExternalID == "7" {
				work = `{"id":9001}`
			}
			out = append(out, `{"external_id":"`+it.ExternalID+`","work":`+work+`}`)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
			strings.Join(out, ",") + `]}}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "nm_test_key", "")
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{7})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if ids[7] != 9001 {
		t.Errorf("gid 7 resolved to %d, want 9001 via the pre-rename source key", ids[7])
	}
	for _, key := range anchorSourceKeys {
		if !slices.Contains(asked, key) {
			t.Errorf("source key %q was never asked for; the lookup only resolves "+
				"on the side of the rename it happens to name", key)
		}
	}
}

// And the post-rename side resolves too, from a cold cache.
func TestAnchorLookupResolvesAfterTheRename(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Items []struct {
				Source     string `json:"source"`
				ExternalID string `json:"external_id"`
			} `json:"items"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		out := make([]string, 0, len(body.Items))
		for _, it := range body.Items {
			work := "null"
			if it.Source == "curated" && it.ExternalID == "8" {
				work = `{"id":9002}`
			}
			out = append(out, `{"external_id":"`+it.ExternalID+`","work":`+work+`}`)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
			strings.Join(out, ",") + `]}}`))
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "nm_test_key", "")
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{8})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if ids[8] != 9002 {
		t.Errorf("gid 8 resolved to %d, want 9002 via the post-rename source key", ids[8])
	}
}

// The registry issues kungal's ids now, and a work minted that way carries NO
// external_ref anchor — an anchor records the id an upstream issued, and a
// brand-new submission has no upstream. The anchor lookup therefore answers
// nothing for it, and answers it as "no such work" rather than as an error, so
// every page of every new entry would 404 in silence.
//
// These pin the identity route that closes it, and the round-trip check that
// makes the route safe.

// adoptedStub serves the two bridge calls: the anchor lookup finds nothing, and
// the works fetch returns whatever `rows` says.
func adoptedStub(t *testing.T, rows map[int64]string) *GalgameClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/catalog/lookup/batch"):
			var body struct {
				Items []struct {
					ExternalID string `json:"external_id"`
				} `json:"items"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			out := make([]string, 0, len(body.Items))
			for _, it := range body.Items {
				out = append(out, `{"external_id":"`+it.ExternalID+`","work":null}`)
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
				strings.Join(out, ",") + `]}}`))
		case strings.HasSuffix(req.URL.Path, "/catalog/works"):
			var items []string
			for _, raw := range strings.Split(req.URL.Query().Get("ids"), ",") {
				var id int64
				for _, r := range strings.TrimSpace(raw) {
					if r >= '0' && r <= '9' {
						id = id*10 + int64(r-'0')
					}
				}
				if frag, ok := rows[id]; ok {
					items = append(items, frag)
				}
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
				strings.Join(items, ",") + `],"next_cursor":null}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "nm_test_key", "")
}

// The adopted case: the claim's product id IS the work's own id, so the row
// points back at what was asked for.
func TestAdoptedIDResolvesWithoutAnAnchor(t *testing.T) {
	c := adoptedStub(t, map[int64]string{
		90210: `{"id":90210,"claimed_by":{"site":"kungal","work_id":90210,"state":"pending"}}`,
	})
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{90210})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if ids[90210] != 90210 {
		t.Errorf("adopted id resolved to %d, want 90210 — a submission with no "+
			"anchor must still reach its own page", ids[90210])
	}
}

// The reason the route needs a guard at all: a legacy gid is a syntactically
// valid work id too. Resolving it by reading that work would hand back a
// DIFFERENT game, and nothing would report an error.
func TestIdentityRouteRefusesAWorkThatNamesSomethingElse(t *testing.T) {
	c := adoptedStub(t, map[int64]string{
		// Work 42 exists and is ours, but its claim points at gid 7 — so work
		// 42 is NOT the entry gid 42 refers to.
		42: `{"id":42,"claimed_by":{"site":"kungal","work_id":7,"state":"live"}}`,
	})
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{42})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if _, found := ids[42]; found {
		t.Errorf("gid 42 resolved to work %d — the round-trip check must reject a "+
			"work whose claim names another id", ids[42])
	}
}

// Another product's claim is not ours to resolve, however the ids line up.
func TestIdentityRouteRefusesAForeignClaim(t *testing.T) {
	c := adoptedStub(t, map[int64]string{
		500: `{"id":500,"claimed_by":{"site":"moyu","work_id":500,"state":"live"}}`,
	})
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{500})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if _, found := ids[500]; found {
		t.Error("a foreign tenant's claim must not resolve as a kungal gid")
	}
}

// An unclaimed registry row is not an entry either.
func TestIdentityRouteRefusesAnUnclaimedWork(t *testing.T) {
	c := adoptedStub(t, map[int64]string{
		600: `{"id":600,"claimed_by":null}`,
	})
	ids, appErr := c.catalogIDsForGIDs(t.Context(), []int{600})
	if appErr != nil {
		t.Fatalf("catalogIDsForGIDs: %v", appErr)
	}
	if _, found := ids[600]; found {
		t.Error("an unclaimed work must not resolve as a kungal gid")
	}
}
