package service

// The tag browse list applies two rules the catalog cannot be asked for — drop
// the do-not-display tier, drop adult terms from a SFW reader — so it pages
// AFTER filtering, out of the precomputed index.
//
// Pinned because the failure mode is a quiet one: filter after the cut instead
// of before and the list still renders, just short a third of its rows, under a
// `total` that promises pages which come back empty. That is exactly what the
// classification wave turned from a rounding error into the common case.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kun-galgame-api/internal/galgame/client"
)

// tagLaneStub answers the tag browse lane in two keyset pages, so the walk's
// cursor handling is exercised rather than assumed.
func tagLaneStub(t *testing.T) *httptest.Server {
	t.Helper()
	row := func(id int, name, tier string, sexual bool, works int) map[string]any {
		return map[string]any{
			"id": id, "name": name, "tier": tier, "kind": "content",
			"sexual": sexual, "work_count": works,
		}
	}
	page1 := []map[string]any{
		row(1, "青梅竹马", "core", false, 300),
		row(2, "游戏", "hidden", false, 9000), // junk truism: on everything, means nothing
		row(3, "陵辱", "core", true, 500),
	}
	page2 := []map[string]any{
		row(4, "校园", "core", false, 400),
		row(5, "PC", "hidden", false, 8000),
		row(6, "触手", "core", true, 100),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(req.URL.Path, "/catalog/tags") {
			http.NotFound(w, req)
			return
		}
		items, next := page1, "p2"
		if req.URL.Query().Get("cursor") == "p2" {
			items, next = page2, ""
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "ok",
			"data": map[string]any{
				"items": items, "next_cursor": next,
				// Upstream's own count, which includes everything this list
				// drops — the number the list must NOT publish as its total.
				"total": len(page1) + len(page2),
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func tagPage(t *testing.T, svc *TagService, isSFW bool, q url.Values) ([]string, int64) {
	t.Helper()
	page, appErr := svc.GetList(context.Background(), q, isSFW)
	if appErr != nil {
		t.Fatalf("GetList: %v", appErr)
	}
	names := make([]string, 0, len(page.Tags))
	for _, tag := range page.Tags {
		names = append(names, tag.Name)
	}
	return names, page.Total
}

func TestTagList_FiltersBeforePagingAndTotalsWhatItServes(t *testing.T) {
	svc := NewTagService(client.New(tagLaneStub(t).URL, "nm_test_key", ""), nil, nil)

	// Most-used first, with the junk tier gone — the two rows that would
	// otherwise head the list on work_count alone are the ones upstream parked
	// in the do-not-display tier.
	open, total := tagPage(t, svc, false, url.Values{})
	want := []string{"陵辱", "校园", "青梅竹马", "触手"}
	if strings.Join(open, ",") != strings.Join(want, ",") {
		t.Errorf("open list = %v, want %v", open, want)
	}
	if total != 4 {
		t.Errorf("open total = %d, want 4 — never upstream's 6", total)
	}

	sfw, sfwTotal := tagPage(t, svc, true, url.Values{})
	if strings.Join(sfw, ",") != "校园,青梅竹马" {
		t.Errorf("SFW list = %v, want the two non-adult terms", sfw)
	}
	if sfwTotal != 2 {
		t.Errorf("SFW total = %d, want 2 — the total moves with the gate, or the pager lies", sfwTotal)
	}
}

// A page comes back FULL up to the last one. Filtering after the cut used to
// hand back a page of two where three were asked for, and the caller has no way
// to tell that from "the list ended here".
func TestTagList_PagesAreFull(t *testing.T) {
	svc := NewTagService(client.New(tagLaneStub(t).URL, "nm_test_key", ""), nil, nil)

	first, total := tagPage(t, svc, false, url.Values{"page": {"1"}, "limit": {"3"}})
	if len(first) != 3 {
		t.Errorf("page 1 of 3 = %v, want three rows", first)
	}
	second, _ := tagPage(t, svc, false, url.Values{"page": {"2"}, "limit": {"3"}})
	if len(second) != 1 {
		t.Errorf("page 2 of 3 = %v, want the one remaining row", second)
	}
	// And the pager's own arithmetic lands on a page that exists.
	if last := (total + 2) / 3; last != 2 {
		t.Errorf("total %d implies %d pages, want 2", total, last)
	}
	beyond, _ := tagPage(t, svc, false, url.Values{"page": {"3"}, "limit": {"3"}})
	if len(beyond) != 0 {
		t.Errorf("page 3 = %v, want nothing", beyond)
	}
}
