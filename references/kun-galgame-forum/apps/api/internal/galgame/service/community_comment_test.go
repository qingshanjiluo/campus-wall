package service

import (
	"testing"

	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/userclient"
)

// TestVisibleTo pins the held-self-see rule: a held (TL0) post is visible only to
// its own author; visible + tombstoned posts always pass.
func TestVisibleTo(t *testing.T) {
	const author int64 = 7
	cases := []struct {
		name   string
		status int32
		viewer int
		want   bool
	}{
		{"visible seen by anyone", communityclient.PostVisible, 0, true},
		{"deleted seen by anyone", communityclient.PostDeleted, 0, true},
		{"held seen by its author", communityclient.PostHeld, 7, true},
		{"held hidden from others", communityclient.PostHeld, 8, false},
		{"held hidden from anon", communityclient.PostHeld, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibleTo(tc.status, author, tc.viewer); got != tc.want {
				t.Errorf("visibleTo(%d, %d, %d) = %v, want %v", tc.status, author, tc.viewer, got, tc.want)
			}
		})
	}
}

func sampleAuthor() userclient.User {
	return userclient.User{ID: 7, Name: "kun", Avatar: "a.webp", Status: 0}
}

// TestBuildCommunityItemDeleted proves a tombstone's body is blanked and the
// placeholder flags are set.
func TestBuildCommunityItemDeleted(t *testing.T) {
	p := communityclient.PostView{ID: 100, AuthorID: 7, ContentRaw: "secret", Status: communityclient.PostDeleted, CreatedAt: "2026-07-13T00:00:00Z"}
	item := buildCommunityItem(p, 42, sampleAuthor(), 0, false)
	if item.Content != "" {
		t.Errorf("deleted content = %q, want blank", item.Content)
	}
	if !item.Deleted || item.Held {
		t.Errorf("deleted=%v held=%v, want deleted=true held=false", item.Deleted, item.Held)
	}
	if item.GalgameID != 42 || item.ID != 100 {
		t.Errorf("item = %+v", item)
	}
}

// TestBuildCommunityItemHeld proves a held post keeps its body (its author will
// see it) and carries held=true; the reply/root pointers project to null when 0.
func TestBuildCommunityItemHeld(t *testing.T) {
	p := communityclient.PostView{ID: 101, AuthorID: 7, ContentRaw: "hi", Status: communityclient.PostHeld, ReplyToPostID: 0, RootPostID: 0}
	item := buildCommunityItem(p, 42, sampleAuthor(), 3, true)
	if item.Content != "hi" || !item.Held || item.Deleted {
		t.Errorf("held item = %+v", item)
	}
	if item.ParentCommentID != nil || item.RootCommentID != nil {
		t.Errorf("top-level pointers should be nil, got parent=%v root=%v", item.ParentCommentID, item.RootCommentID)
	}
	if item.LikeCount != 3 || item.User.ID != 7 {
		t.Errorf("item = %+v", item)
	}
}

// TestBuildCommunityItemPointers proves reply/root ids project as non-nil.
func TestBuildCommunityItemPointers(t *testing.T) {
	p := communityclient.PostView{ID: 102, AuthorID: 7, ContentRaw: "re", ReplyToPostID: 50, RootPostID: 40, EditedAt: "2026-07-13T01:00:00Z", EditedByModerator: true}
	item := buildCommunityItem(p, 42, sampleAuthor(), 0, false)
	if item.ParentCommentID == nil || *item.ParentCommentID != 50 {
		t.Errorf("parent = %v, want 50", item.ParentCommentID)
	}
	if item.RootCommentID == nil || *item.RootCommentID != 40 {
		t.Errorf("root = %v, want 40", item.RootCommentID)
	}
	if item.Edited == nil || *item.Edited != "2026-07-13T01:00:00Z" || !item.EditedByModerator {
		t.Errorf("edited = %v, edited_by_moderator = %v", item.Edited, item.EditedByModerator)
	}
}

// TestLikeEffects pins the triple-write consistency: the local row presence, the
// moemoepoint delta, and the notify flag all derive from the community toggle's
// authoritative added/removed, with self-likes rewarding/notifying nothing.
func TestLikeEffects(t *testing.T) {
	cases := []struct {
		name       string
		added      bool
		authorID   int64
		userID     int
		wantLocal  bool
		wantDelta  int
		wantNotify bool
	}{
		{"add, other", true, 7, 9, true, 1, true},
		{"add, self", true, 9, 9, true, 0, false},
		{"remove, other", false, 7, 9, false, -1, false},
		{"remove, self", false, 9, 9, false, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local, delta, notify := likeEffects(tc.added, tc.authorID, tc.userID)
			if local != tc.wantLocal || delta != tc.wantDelta || notify != tc.wantNotify {
				t.Errorf("likeEffects(%v,%d,%d) = (%v,%d,%v), want (%v,%d,%v)",
					tc.added, tc.authorID, tc.userID, local, delta, notify, tc.wantLocal, tc.wantDelta, tc.wantNotify)
			}
		})
	}
}

// TestClampReadLimit pins the ≤50 read cap and the default.
func TestClampReadLimit(t *testing.T) {
	cases := map[int]string{0: "50", -1: "50", 1: "1", 30: "30", 50: "50", 51: "50", 999: "50"}
	for in, want := range cases {
		if got := clampReadLimit(in); got != want {
			t.Errorf("clampReadLimit(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestParseAnchorGid pins galgame-id parsing from a site_game anchor.
func TestParseAnchorGid(t *testing.T) {
	cases := map[string]int{"42": 42, "": 0, "abc": 0, "0": 0, "-5": 0}
	for in, want := range cases {
		if got := parseAnchorGid(in); got != want {
			t.Errorf("parseAnchorGid(%q) = %d, want %d", in, got, want)
		}
	}
}
