package service

import (
	"testing"

	"kun-galgame-api/pkg/communityclient"
)

// TestResourceAnchorID pins the site_resource anchor construction per area.
func TestResourceAnchorID(t *testing.T) {
	cases := []struct {
		src  CommentSource
		id   int
		want string
	}{
		{sourceRating, 42, "rating:42"},
		{sourceWebsite, 7, "website:7"},
		{sourceToolset, 3, "toolset:3"},
	}
	for _, tc := range cases {
		if got := tc.src.anchorID(tc.id); got != tc.want {
			t.Errorf("%s.anchorID(%d) = %q, want %q", tc.src.key, tc.id, got, tc.want)
		}
	}
}

// TestResourceSourceMetadata pins the accessor→strategy wiring and the feed types
// that the create/delete paths must project (charter ruling 22).
func TestResourceSourceMetadata(t *testing.T) {
	cases := []struct {
		src      CommentSource
		key      string
		feedType string
	}{
		{SourceRating(), "rating", "GALGAME_RATING_COMMENT_CREATION"},
		{SourceWebsite(), "website", "GALGAME_WEBSITE_COMMENT_CREATION"},
		{SourceToolset(), "toolset", "TOOLSET_COMMENT_CREATION"},
	}
	for _, tc := range cases {
		if tc.src.key != tc.key || tc.src.feedType != tc.feedType {
			t.Errorf("source %+v, want key=%q feedType=%q", tc.src, tc.key, tc.feedType)
		}
	}
}

// post is a tiny PostView builder for the notification-plan cases.
func post(replyTo, target int64) *communityclient.PostView {
	return &communityclient.PostView{ReplyToPostID: replyTo, TargetUserID: target}
}

// TestResourceNotifyPlan pins the per-area notification parity (charter ruling
// 20): who is notified, the message type, and the suppression rules.
func TestResourceNotifyPlan(t *testing.T) {
	cases := []struct {
		name         string
		src          CommentSource
		sender       int
		post         *communityclient.PostView
		toolsetOwner int
		wantRcv      int
		wantType     string
		wantOK       bool
	}{
		// rating: "commented" → the explicit target; self-notification suppressed.
		{"rating to target", sourceRating, 1, post(0, 2), 0, 2, "commented", true},
		{"rating self-suppress", sourceRating, 1, post(0, 1), 0, 0, "", false},
		{"rating no target", sourceRating, 1, post(0, 0), 0, 0, "", false},

		// website: notify ONLY on a reply, to the parent author.
		{"website top-level notifies nobody", sourceWebsite, 1, post(0, 0), 0, 0, "", false},
		{"website reply to parent", sourceWebsite, 1, post(5, 2), 0, 2, "commented", true},
		{"website reply self-suppress", sourceWebsite, 2, post(5, 2), 0, 0, "", false},

		// toolset: "commented" → owner (top-level), "replied" → parent (reply).
		{"toolset top-level to owner", sourceToolset, 1, post(0, 0), 9, 9, "commented", true},
		{"toolset reply to parent", sourceToolset, 1, post(5, 3), 9, 3, "replied", true},
		{"toolset owner-is-self suppress", sourceToolset, 9, post(0, 0), 9, 0, "", false},
		{"toolset no owner", sourceToolset, 1, post(0, 0), 0, 0, "", false},

		// resource / quiz: introduced on the primitive, so no legacy notifier to
		// match — they deliberately adopt the toolset shape (owner on a top-level
		// comment, parent author on a reply).
		{"resource top-level to uploader", sourceResource, 1, post(0, 0), 9, 9, "commented", true},
		{"resource reply to parent", sourceResource, 1, post(5, 3), 9, 3, "replied", true},
		{"resource uploader-is-self suppress", sourceResource, 9, post(0, 0), 9, 0, "", false},
		{"resource no uploader", sourceResource, 1, post(0, 0), 0, 0, "", false},

		{"quiz top-level to author", sourceQuiz, 1, post(0, 0), 9, 9, "commented", true},
		{"quiz reply to parent", sourceQuiz, 1, post(5, 3), 9, 3, "replied", true},
		{"quiz author-is-self suppress", sourceQuiz, 9, post(0, 0), 9, 0, "", false},
		{"quiz no author", sourceQuiz, 1, post(0, 0), 0, 0, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, ok := resourceNotifyPlan(tc.src, tc.sender, tc.post, tc.toolsetOwner)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (plan %+v)", ok, tc.wantOK, plan)
			}
			if ok && (plan.receiver != tc.wantRcv || plan.msgType != tc.wantType) {
				t.Errorf("plan = %+v, want receiver=%d type=%q", plan, tc.wantRcv, tc.wantType)
			}
		})
	}
}

// TestContainsPost pins the owner-delete membership scan's element check.
func TestContainsPost(t *testing.T) {
	posts := []communityclient.PostView{{ID: 10}, {ID: 20}, {ID: 30}}
	if !containsPost(posts, 20) {
		t.Error("containsPost should find 20")
	}
	if containsPost(posts, 99) {
		t.Error("containsPost should not find 99")
	}
	if containsPost(nil, 1) {
		t.Error("containsPost(nil) should be false")
	}
}
