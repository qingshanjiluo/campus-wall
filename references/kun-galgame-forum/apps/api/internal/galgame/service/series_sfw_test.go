package service

// The series index is gated as a WHOLE entity, not per member.
//
// A series is a grouping, so the per-work content gate the rest of the
// catalogue uses would leave a fragment — half a series, with the montage and
// the count disagreeing about which half. The catalog answers the whole-entity
// question with has_nsfw, the aggregate of its members' editorial content_limit
// (NOT the age rating: an r18-rated work whose claim says sfw is shown by this
// product everywhere else).
//
// Pinned because the failure is invisible from this side either way: read the
// flag wrong and the page still renders, just with adult series in front of a
// reader who asked not to see them — or with an index that silently lost
// everything.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-api/internal/galgame/client"
)

// seriesStub answers the series browse lane with three groupings — clean,
// adult, and one with no published member. hasNSFWField=false drops has_nsfw
// entirely, standing in for a catalog that predates it.
func seriesStub(t *testing.T, hasNSFWField bool) *httptest.Server {
	t.Helper()
	row := func(id int, name string, count int, nsfw bool) map[string]any {
		r := map[string]any{"id": id, "display_name": name, "work_count": count}
		if hasNSFWField {
			r["has_nsfw"] = nsfw
		}
		return r
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(req.URL.Path, "/catalog/series") {
			items := []map[string]any{
				row(1, "全年龄系列", 2, false),
				row(2, "成人系列", 3, true),
				row(3, "无已发布作品", 0, false),
				// Upstream counts two live members, but neither is claimed by
				// this product, so nothing here can open them.
				row(4, "无可展示成员", 2, false),
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "message": "ok",
				"data": map[string]any{"items": items, "next_cursor": "", "total": len(items)},
			})
			return
		}
		// Members, so each series has something listable behind it. No local
		// repository is wired here, so every claimed member counts (see
		// listableGIDs) — this stub is about the content rule, not the
		// "本站已收录" one.
		member := func(gid int) map[string]any {
			return map[string]any{
				"id": gid + 1000, "display_name": "作品",
				"claimed_by": map[string]any{"site": "kungal", "work_id": gid, "state": "live"},
			}
		}
		items := []map[string]any{}
		switch req.URL.Query().Get("series_id") {
		case "1":
			items = append(items, member(11), member(12))
		case "2":
			items = append(items, member(21), member(22), member(23))
		case "4":
			items = append(items, map[string]any{"id": 4001, "display_name": "别处的作品"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "ok",
			"data": map[string]any{"items": items, "total": len(items)},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func seriesNames(t *testing.T, svc *SeriesService, isSFW bool) []string {
	t.Helper()
	page, appErr := svc.GetCards(context.Background(), nil, 1, 12, isSFW)
	if appErr != nil {
		t.Fatalf("GetCards: %v", appErr)
	}
	names := make([]string, 0, len(page.Series))
	for _, c := range page.Series {
		names = append(names, c.Name)
	}
	if int(page.Total) != len(names) {
		t.Errorf("total = %d but %d rows — the pager promises pages that filter away", page.Total, len(names))
	}
	return names
}

func TestSeriesIndex_HidesAdultSeriesFromSFWReaders(t *testing.T) {
	svc := NewSeriesService(client.New(seriesStub(t, true).URL, "nm_test_key", ""), nil)

	if got := seriesNames(t, svc, true); len(got) != 1 || got[0] != "全年龄系列" {
		t.Errorf("SFW index = %v, want only the series with no adult member", got)
	}
	// The grouping with no published member stays out of BOTH views: the page
	// behind it has nothing to list.
	if got := seriesNames(t, svc, false); len(got) != 2 {
		t.Errorf("open index = %v, want both series that have published members", got)
	}
}

// A series whose members this site cannot list is not listed either: the card
// would advertise games and then lead to an empty page — which is what series
// 38 did (one live member, claimed on the wiki, never given a local row).
func TestSeriesIndex_HidesSeriesWithNothingListable(t *testing.T) {
	svc := NewSeriesService(client.New(seriesStub(t, true).URL, "nm_test_key", ""), nil)

	for _, name := range seriesNames(t, svc, false) {
		if name == "无可展示成员" {
			t.Errorf("open index lists %q, whose page has nothing to show", name)
		}
	}
}

// An unanswered has_nsfw is not "clean". A catalog that predates the field
// would otherwise report every series as safe, which is the one reading that
// shows adult work to a reader who opted out.
func TestSeriesIndex_UnansweredHasNSFWIsNotSafe(t *testing.T) {
	svc := NewSeriesService(client.New(seriesStub(t, false).URL, "nm_test_key", ""), nil)

	if got := seriesNames(t, svc, true); len(got) != 0 {
		t.Errorf("SFW index = %v, want nothing — the catalog answered nothing", got)
	}
	// The open view is unaffected: the flag gates only the SFW reader.
	if got := seriesNames(t, svc, false); len(got) != 2 {
		t.Errorf("open index = %v, want both series that have published members", got)
	}
}
