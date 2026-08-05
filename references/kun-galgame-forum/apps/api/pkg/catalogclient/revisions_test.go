package catalogclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The revision feed's cursor contract is the whole reason the activity sync can
// be idempotent, so the request it issues is pinned here: exclusive `since`,
// explicit `limit`, and the entity_type narrowing that keeps a global feed from
// dumping other families onto the galgame timeline.
func TestEditRevisionsSince_RequestAndParse(t *testing.T) {
	var gotPath, gotSince, gotLimit, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSince = r.URL.Query().Get("since")
		gotLimit = r.URL.Query().Get("limit")
		gotType = r.URL.Query().Get("entity_type")
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"items":[
			{"id":41,"entity_family":"galgame","entity_type":"galgame.game","entity_id":1207,
			 "seq":8,"action":1,"changed_fields":["galgame.game.name_ja_jp"],
			 "actor_uid":9,"amender_uid":null,"proposal_id":77,"site":"kungal",
			 "created_at":"2026-07-30T10:00:00Z"}],"next_since":41}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
	page, err := c.EditRevisionsSince(context.Background(), 40, 1000, "galgame.game")
	if err != nil {
		t.Fatalf("EditRevisionsSince: %v", err)
	}

	if gotPath != "/api/v1/catalog/edit-revisions/feed" {
		t.Errorf("path = %q", gotPath)
	}
	if gotSince != "40" || gotLimit != "1000" || gotType != "galgame.game" {
		t.Errorf("query = since:%q limit:%q entity_type:%q", gotSince, gotLimit, gotType)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(page.Items))
	}
	got := page.Items[0]
	// entity_id is the gid and seq is the per-galgame revision number: the two
	// fields the activity row is built from. Losing either silently produces a
	// card that cannot render its diff.
	if got.ID != 41 || got.EntityID != 1207 || got.Seq != 8 || got.ActorUID != 9 {
		t.Errorf("item = %+v", got)
	}
	if got.Action != EditActionMerged {
		t.Errorf("action = %d, want merged", got.Action)
	}
	if page.NextSince != 41 {
		t.Errorf("next_since = %d, want 41", page.NextSince)
	}
}

// An empty page must not look like an error, and its next_since echoes the
// request so a consumer storing it unconditionally never rewinds.
func TestEditRevisionsSince_EmptyPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"成功","data":{"items":[],"next_since":12681}}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec"})
	page, err := c.EditRevisionsSince(context.Background(), 12681, 1000, "galgame.game")
	if err != nil {
		t.Fatalf("EditRevisionsSince: %v", err)
	}
	if len(page.Items) != 0 || page.NextSince != 12681 {
		t.Errorf("page = %+v", page)
	}
}

// An unconfigured client must fail loudly rather than report an empty feed —
// an empty feed would look like "nothing changed" and hold the cron silent.
func TestEditRevisionsSince_NotConfigured(t *testing.T) {
	if _, err := New(Config{}).EditRevisionsSince(context.Background(), 0, 10, ""); err == nil {
		t.Fatal("want an error from an unconfigured client")
	}
}
