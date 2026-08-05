package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"kun-galgame-api/pkg/catalogclient"
)

// revFeedStub serves the engine revision feed as a paginated, ascending,
// exclusive-cursor stream over `total` rows, recording every `since` it was
// asked for.
type revFeedStub struct {
	mu    sync.Mutex
	total int64
	page  int
	asked []int64
}

func (f *revFeedStub) server(t *testing.T) *catalogclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		f.mu.Lock()
		f.asked = append(f.asked, since)
		f.mu.Unlock()

		items := []string{}
		for id := since + 1; id <= f.total && len(items) < f.page; id++ {
			items = append(items, fmt.Sprintf(
				`{"id":%d,"entity_family":"galgame","entity_type":"galgame.game",`+
					`"entity_id":%d,"seq":2,"action":1,"changed_fields":[],`+
					`"actor_uid":5,"amender_uid":null,"proposal_id":null,`+
					`"site":"kungal","created_at":"2026-07-30T10:00:00Z"}`, id, 1000+id))
		}
		next := since
		if n := len(items); n > 0 {
			next = since + int64(n)
		}
		_, _ = fmt.Fprintf(w, `{"code":0,"message":"成功","data":{"items":[%s],"next_since":%d}}`,
			strings.Join(items, ","), next)
	}))
	t.Cleanup(srv.Close)
	return catalogclient.New(catalogclient.Config{
		BaseURL: srv.URL, ClientID: "cid", ClientSecret: "sec",
	})
}

// feedHead is what protects the timeline from being rewritten on the first run
// after the switch: the engine's table holds the whole imported edit history,
// so a fresh cursor must land on the CURRENT last id — walking every page and
// stopping on a short one — rather than at 0.
func TestFeedHeadWalksToTheLastID(t *testing.T) {
	stub := &revFeedStub{total: 250, page: 100}
	s := NewGalgameEditRevisionSync(stub.server(t), nil, nil, nil)
	s.batch = stub.page

	head, err := s.feedHead(context.Background())
	if err != nil {
		t.Fatalf("feedHead: %v", err)
	}
	if head != 250 {
		t.Errorf("head = %d, want 250 (the feed's last id)", head)
	}
	// 0 → 100 → 200 → 250(short page, stop). A cursor that failed to advance
	// would loop until the page cap and hammer the service.
	stub.mu.Lock()
	defer stub.mu.Unlock()
	want := []int64{0, 100, 200}
	if len(stub.asked) != len(want) {
		t.Fatalf("asked = %v, want %v", stub.asked, want)
	}
	for i := range want {
		if stub.asked[i] != want[i] {
			t.Fatalf("asked = %v, want %v", stub.asked, want)
		}
	}
}

// An empty feed seeds at 0 without erroring: a kungal instance whose engine has
// no galgame revisions yet must still start syncing from the beginning.
func TestFeedHeadOnEmptyFeed(t *testing.T) {
	stub := &revFeedStub{total: 0, page: 100}
	s := NewGalgameEditRevisionSync(stub.server(t), nil, nil, nil)
	s.batch = stub.page

	head, err := s.feedHead(context.Background())
	if err != nil {
		t.Fatalf("feedHead: %v", err)
	}
	if head != 0 {
		t.Errorf("head = %d, want 0", head)
	}
}

// The action filter decides what a timeline card can claim. `created` is the
// entity's birth, not an edit — 9,694 of the engine's 12,581 galgame revisions
// are creations, so admitting them would flood the feed with cards for games
// nobody edited.
func TestOnlyEditsReachTheTimeline(t *testing.T) {
	cases := []struct {
		action int16
		want   bool
		name   string
	}{
		{catalogclient.EditActionCreated, false, "created"},
		{catalogclient.EditActionMerged, true, "merged"},
		{catalogclient.EditActionDirect, true, "direct"},
		{catalogclient.EditActionReverted, true, "reverted"},
	}
	for _, c := range cases {
		if got := isTimelineEdit(c.action); got != c.want {
			t.Errorf("isTimelineEdit(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}

// The wire shape the sync depends on is the engine's, not the wiki's: entity_id
// carries the gid and seq carries the per-galgame revision number. This pins
// the two so a contract drift upstream fails here rather than as a timeline of
// cards whose diff link 404s.
func TestFeedItemCarriesGIDAndRevisionNumber(t *testing.T) {
	var item catalogclient.EditRevisionFeedItem
	raw := `{"id":41,"entity_family":"galgame","entity_type":"galgame.game",
		"entity_id":1207,"seq":8,"action":2,"changed_fields":["galgame.game.name_ja_jp"],
		"actor_uid":9,"amender_uid":3,"proposal_id":null,"site":"kungal",
		"created_at":"2026-07-30T10:00:00Z"}`
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if item.EntityID != 1207 {
		t.Errorf("entity_id = %d, want the gid 1207", item.EntityID)
	}
	if item.Seq != 8 {
		t.Errorf("seq = %d, want the revision number 8", item.Seq)
	}
	if item.ActorUID != 9 {
		t.Errorf("actor_uid = %d — the card attributes the edit to the PROPOSER", item.ActorUID)
	}
}
