package client

// Contract tests for the detail projection (A2-R2). Hermetic: the aggregate is
// served by a httptest catalog, so the whole path — wire decode plus projection
// — is under test rather than a hand-built struct.
//
// What they pin, and why each is a test rather than a comment:
//
//   - labels[] is one row per attribution EDGE. A brand credited as developer
//     AND publisher AND brand arrives three times, and the un-deduped strip
//     rendered the same 会社 three times on the live detail page;
//   - the chip's category must be the label's OWN kind (label_kind), because
//     that is the vocabulary the 会社 index page's map speaks — the per-edge
//     role leaked raw English words like "developer" onto the page;
//   - work_count reaches all three chip families. The projection used to leave
//     galgame_count at its zero value, so every chip on the site read "+ 0";
//   - a MISSING work_count key is 0, not an error and not a guess. Two real
//     cases produce one: the window before the supplying wave is deployed, and
//     an unmapped source tag, which never gets a count at all.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-api/internal/galgame/dto"
)

// detailStub serves the two hops CatalogWorkDetail makes: the gid→catalog id
// lookup, then the aggregate itself. body is the `data` object of the detail
// response, so a test writes only the blocks it cares about.
func detailStub(t *testing.T, gid int, catalogID int64, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(req.URL.Path, "/catalog/lookup/batch"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"items":[` +
				`{"source":"curated","external_id":"` + itoa(int64(gid)) +
				`","type":"work","work":{"id":` + itoa(catalogID) + `}}]}}`))
		case req.URL.Path == "/v1/catalog/works/"+itoa(catalogID):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":` + body + `}`))
		default:
			t.Errorf("unexpected upstream call: %s", req.URL.Path)
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fullOf drives the real two-hop fetch and returns the projected detail.
func fullOf(t *testing.T, body string) dto.NextMoeGalgameDetailFull {
	t.Helper()
	const gid, catalogID = 777, 4242
	srv := detailStub(t, gid, catalogID, body)
	c := New(srv.URL, "nm_test_key", "")

	d, found, appErr := c.CatalogWorkDetail(context.Background(), gid)
	if appErr != nil {
		t.Fatalf("CatalogWorkDetail: %v", appErr)
	}
	if !found {
		t.Fatal("CatalogWorkDetail: not found, want the stubbed work")
	}
	return CatalogDetailToFull(d, gid)
}

// TestCatalogDetail_HeroPrefersTheLandscapeCover pins the detail hero against
// the card: both must resolve to the wide art. The detail face ships a flat
// covers[] ordered portrait-pin-first, so taking covers[0] — what this used to
// do — put a portrait in a landscape hero frame for every work that has both.
func TestCatalogDetail_HeroPrefersTheLandscapeCover(t *testing.T) {
	t.Run("landscape present", func(t *testing.T) {
		body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja","covers":[
			{"url":"https://cdn.example/ab/cd/portrait.webp","kind":"main","width":600,"height":800,"thumbhash":"P"},
			{"url":"https://cdn.example/ef/gh/banner.webp","kind":"dig","width":1280,"height":720,"thumbhash":"B"}
		]}`
		f := fullOf(t, body)
		if f.EffectiveBannerURL != "https://cdn.example/ef/gh/banner.webp" {
			t.Errorf("hero = %q, want the landscape cover", f.EffectiveBannerURL)
		}
		if f.EffectiveBannerWidth != 1280 || f.EffectiveBannerHeight != 720 || f.EffectiveBannerThumbhash != "B" {
			t.Errorf("dims/thumbhash = %dx%d %q, want 1280x720 B",
				f.EffectiveBannerWidth, f.EffectiveBannerHeight, f.EffectiveBannerThumbhash)
		}
		// The gallery itself is untouched — only the derived hero moved.
		if len(f.Covers) != 2 || f.Covers[0].CDNURL != "https://cdn.example/ab/cd/portrait.webp" {
			t.Errorf("covers[] order changed: %+v", f.Covers)
		}
	})

	t.Run("portrait only falls back", func(t *testing.T) {
		body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja","covers":[
			{"url":"https://cdn.example/ab/cd/portrait.webp","kind":"main","width":600,"height":800,"thumbhash":"P"}
		]}`
		f := fullOf(t, body)
		if f.EffectiveBannerURL != "https://cdn.example/ab/cd/portrait.webp" {
			t.Errorf("hero = %q, want the portrait fallback rather than an empty hero", f.EffectiveBannerURL)
		}
	})

	t.Run("dims unknown falls back to the pin", func(t *testing.T) {
		// image_service has no entry for either cover, so orientation is
		// unknowable — the server-ordered pin is the honest choice.
		body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja","covers":[
			{"url":"https://cdn.example/ab/cd/pinned.webp","kind":"main"},
			{"url":"https://cdn.example/ef/gh/other.webp","kind":"dig"}
		]}`
		f := fullOf(t, body)
		if f.EffectiveBannerURL != "https://cdn.example/ab/cd/pinned.webp" {
			t.Errorf("hero = %q, want the pinned cover when no dims are known", f.EffectiveBannerURL)
		}
	})
}

func TestCatalogDetail_LabelsDedupPerLabelID(t *testing.T) {
	// The same brand on three edges, plus a second label — the shape a doujin
	// work with a circle and a publisher actually has.
	body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja","labels":[
		{"id":11,"display_name":"戯画","label_kind":"game_brand","kind":"developer","lang":"ja","work_count":42},
		{"id":11,"display_name":"戯画","label_kind":"game_brand","kind":"publisher","lang":"ja","work_count":42},
		{"id":11,"display_name":"戯画","label_kind":"game_brand","kind":"brand","lang":"ja","work_count":42},
		{"id":22,"display_name":"Circle","label_kind":"doujin_circle","kind":"circle","lang":"en","work_count":3}
	]}`
	f := fullOf(t, body)

	if len(f.Official) != 2 {
		t.Fatalf("projected %d 制作方 rows, want 2 — one per LABEL, not one per attribution edge: %+v",
			len(f.Official), f.Official)
	}
	// First-seen order survives: the face orders edges by kind, so the winner
	// must be stable rather than map-ordered.
	if f.Official[0].Official.ID != 11 || f.Official[1].Official.ID != 22 {
		t.Errorf("row order = [%d %d], want [11 22] (first occurrence wins)",
			f.Official[0].Official.ID, f.Official[1].Official.ID)
	}
	// The chip's category is the label's own kind — the vocabulary the 会社
	// index page's map speaks. The per-edge role would print "developer".
	if got := f.Official[0].Official.Category; got != "game_brand" {
		t.Errorf("category = %q, want game_brand (label_kind, not the edge kind)", got)
	}
	if got := f.Official[1].Official.Category; got != "doujin_circle" {
		t.Errorf("category = %q, want doujin_circle", got)
	}
	if got := f.Official[0].Official.Lang; got != "ja" {
		t.Errorf("lang = %q, want ja (an empty lang renders an empty span)", got)
	}
	if got := f.Official[0].Official.GalgameCount; got != 42 {
		t.Errorf("galgame_count = %d, want 42", got)
	}
}

func TestCatalogDetail_WorkCountReachesAllThreeChipFamilies(t *testing.T) {
	body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja",
		"labels":[{"id":11,"display_name":"戯画","label_kind":"game_brand","kind":"developer","lang":"ja","work_count":42}],
		"engines":[{"id":31,"name":"KiriKiri","work_count":1337}],
		"tags":[{"name":"純愛","canonical_id":51,"kind":"content","spoiler":0,"sexual":false,"work_count":9001}]}`
	f := fullOf(t, body)

	if len(f.Official) != 1 || f.Official[0].Official.GalgameCount != 42 {
		t.Errorf("label count not projected: %+v", f.Official)
	}
	if len(f.Engine) != 1 || f.Engine[0].Engine.GalgameCount != 1337 {
		t.Errorf("engine count not projected: %+v", f.Engine)
	}
	if len(f.Tag) != 1 || f.Tag[0].Tag.GalgameCount != 9001 {
		t.Errorf("tag count not projected: %+v", f.Tag)
	}
}

func TestCatalogDetail_MissingWorkCountKeyIsZero(t *testing.T) {
	// Exactly the pre-deployment aggregate: no work_count key anywhere. The
	// unmapped tag is the permanent version of the same case (it is dropped
	// from the strip entirely, so only the mapped one is asserted on).
	body := `{"id":4242,"display_name":"Kun","content_rating":"all_ages","olang":"ja",
		"labels":[{"id":11,"display_name":"戯画","label_kind":"game_brand","kind":"developer","lang":"ja"}],
		"engines":[{"id":31,"name":"KiriKiri"}],
		"tags":[{"name":"純愛","canonical_id":51,"kind":"content","spoiler":0,"sexual":false},
		        {"name":"unmapped","canonical_id":0,"kind":"content","spoiler":0,"sexual":false}]}`
	f := fullOf(t, body)

	if len(f.Official) != 1 || f.Official[0].Official.GalgameCount != 0 {
		t.Errorf("missing label work_count must decode to 0, got %+v", f.Official)
	}
	if len(f.Engine) != 1 || f.Engine[0].Engine.GalgameCount != 0 {
		t.Errorf("missing engine work_count must decode to 0, got %+v", f.Engine)
	}
	if len(f.Tag) != 1 {
		t.Fatalf("projected %d tag chips, want 1 (the unmapped row has no id to link to): %+v", len(f.Tag), f.Tag)
	}
	if f.Tag[0].Tag.GalgameCount != 0 {
		t.Errorf("missing tag work_count must decode to 0, got %+v", f.Tag[0])
	}
	// The rows themselves must still render — a missing count hides the badge,
	// never the 会社/引擎/标签 itself.
	if f.Official[0].Official.Name != "戯画" || f.Engine[0].Engine.Name != "KiriKiri" {
		t.Errorf("rows dropped along with their counts: %+v %+v", f.Official, f.Engine)
	}
}
