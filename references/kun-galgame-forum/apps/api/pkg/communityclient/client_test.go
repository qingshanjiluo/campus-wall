package communityclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kun-galgame-api/pkg/communityclient"
)

func newTestClient(baseURL string) *communityclient.Client {
	return communityclient.New(communityclient.Config{BaseURL: baseURL, ClientID: "cid", ClientSecret: "sec"})
}

// TestNotConfigured proves an unconfigured client degrades: Configured() is
// false and every call short-circuits to ErrNotConfigured with no network hit.
func TestNotConfigured(t *testing.T) {
	c := communityclient.New(communityclient.Config{BaseURL: "http://x"}) // no creds
	if c.Configured() {
		t.Fatal("Configured() true without creds")
	}
	if _, err := c.ResolveComments(context.Background(), communityclient.ResolveCommentsRequest{}); !errors.Is(err, communityclient.ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

// TestResolveEnvelopeAndAuth checks the Basic auth header, the request path, and
// that the {code,message,data} envelope decodes into the typed result.
func TestResolveEnvelopeAndAuth(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "成功",
			"data": map[string]any{
				"thread": map[string]any{"id": 7, "site": "kungal", "kind": 1, "anchor_kind": 1, "anchor_id": "42", "content_rating": 0, "status": 0, "posts_count": 2, "participants_count": 1, "highest_post_number": 2, "created_by": 1, "created_at": "2026-07-13T00:00:00Z"},
				"posts": []any{
					map[string]any{"id": 100, "thread_id": 7, "post_number": 1, "author_id": 1, "content_raw": "hi", "content_html": "<p>hi</p>", "content_rating": 0, "status": 0, "created_at": "2026-07-13T00:00:00Z"},
				},
				"next_cursor": "",
			},
		})
	}))
	defer srv.Close()

	out, err := newTestClient(srv.URL).ResolveComments(context.Background(), communityclient.ResolveCommentsRequest{
		AnchorKind: communityclient.AnchorSiteGame, AnchorID: "42", ContentRating: communityclient.RatingAll,
	})
	if err != nil {
		t.Fatalf("ResolveComments: %v", err)
	}
	if gotAuth != "Basic Y2lkOnNlYw==" { // base64("cid:sec")
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotPath != "/comments/resolve" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotBody, `"anchor_kind":1`) {
		t.Errorf("body %q missing anchor_kind", gotBody)
	}
	if out.Thread.ID != 7 || len(out.Posts) != 1 || out.Posts[0].ID != 100 {
		t.Errorf("decoded = %+v", out)
	}
}

// TestErrorMapping proves 403→ErrForbidden, 429→ErrRateLimited, and a non-zero
// business code (HTTP 200) surfaces as *APIError.
func TestErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   map[string]any
		want   error // sentinel, or nil to expect an *APIError
	}{
		{"forbidden site binding", http.StatusForbidden, nil, communityclient.ErrForbidden},
		{"tl0 rate limit", http.StatusTooManyRequests, nil, communityclient.ErrRateLimited},
		{"business code", http.StatusOK, map[string]any{"code": 40001, "message": "bad"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				if tc.body != nil {
					_ = json.NewEncoder(w).Encode(tc.body)
				}
			}))
			defer srv.Close()

			_, err := newTestClient(srv.URL).ListPosts(context.Background(), 7, "", "")
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Errorf("err = %v, want %v", err, tc.want)
				}
				return
			}
			var apiErr *communityclient.APIError
			if !errors.As(err, &apiErr) || apiErr.Code != 40001 {
				t.Errorf("err = %v, want *APIError code=40001", err)
			}
		})
	}
}

// TestToggleReactionResult proves the reaction result (added + the post's
// author/anchor context) decodes so the BFF can drive the like triple-write.
func TestToggleReactionResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/posts/100/reaction" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"added": true, "author_id": 55, "thread_id": 7, "anchor_kind": 1, "anchor_id": "42"},
		})
	}))
	defer srv.Close()

	res, err := newTestClient(srv.URL).ToggleReaction(context.Background(), 100, communityclient.ReactionToggleRequest{UserID: 9, Kind: communityclient.ReactionLike})
	if err != nil {
		t.Fatalf("ToggleReaction: %v", err)
	}
	if !res.Added || res.AuthorID != 55 || res.AnchorID != "42" {
		t.Errorf("result = %+v", res)
	}
}

// TestAuthorPosts proves the by-author path, the after/limit/anchor_kind query
// params, and that the nested {post, thread} view + next_cursor decode.
func TestAuthorPosts(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{
				"posts": []any{
					map[string]any{
						"post":   map[string]any{"id": 100, "thread_id": 7, "post_number": 3, "author_id": 55, "content_raw": "hi", "status": 0, "created_at": "2026-07-16T00:00:00Z"},
						"thread": map[string]any{"thread_id": 7, "title": "", "anchor_kind": 1, "anchor_id": "42"},
					},
				},
				"next_cursor": "99",
			},
		})
	}))
	defer srv.Close()

	out, err := newTestClient(srv.URL).AuthorPosts(context.Background(), 55, "150", 20, communityclient.AnchorSiteGame)
	if err != nil {
		t.Fatalf("AuthorPosts: %v", err)
	}
	if gotPath != "/authors/55/posts" {
		t.Errorf("path = %q", gotPath)
	}
	for _, want := range []string{"after=150", "limit=20", "anchor_kind=1"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
	if len(out.Posts) != 1 || out.Posts[0].Post.ID != 100 || out.Posts[0].Thread.AnchorID != "42" || out.NextCursor != "99" {
		t.Errorf("decoded = %+v", out)
	}
}

// TestAuthorStats proves the ids join, the ≤100 batch shape, and the empty-input
// short-circuit (no network hit).
func TestAuthorStats(t *testing.T) {
	var gotQuery string
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit, gotQuery = true, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"stats": []any{
				map[string]any{"author_id": 55, "visible_posts": 9},
				map[string]any{"author_id": 56, "visible_posts": 0},
			}},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	out, err := c.AuthorStats(context.Background(), []int64{55, 56})
	if err != nil {
		t.Fatalf("AuthorStats: %v", err)
	}
	if !strings.Contains(gotQuery, "ids=55%2C56") { // url-encoded "55,56"
		t.Errorf("query %q missing joined ids", gotQuery)
	}
	if len(out.Stats) != 2 || out.Stats[0].VisiblePosts != 9 {
		t.Errorf("decoded = %+v", out)
	}
	// Empty input must not hit the network.
	hit = false
	if res, err := c.AuthorStats(context.Background(), nil); err != nil || len(res.Stats) != 0 || hit {
		t.Errorf("empty AuthorStats hit=%v res=%+v err=%v", hit, res, err)
	}
}

// TestAuthorPurge proves the purge path (POST, no body) and the {posts_purged,
// reactions_deleted} decode.
func TestAuthorPurge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/authors/55/purge" {
			t.Errorf("method/path = %s %q", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"posts_purged": 3, "reactions_deleted": 2},
		})
	}))
	defer srv.Close()

	out, err := newTestClient(srv.URL).AuthorPurge(context.Background(), 55)
	if err != nil {
		t.Fatalf("AuthorPurge: %v", err)
	}
	if out.PostsPurged != 3 || out.ReactionsDeleted != 2 {
		t.Errorf("decoded = %+v", out)
	}
}

// TestResolvePosts proves the ids body, request-order hydration, and the empty-
// input short-circuit.
func TestResolvePosts(t *testing.T) {
	var gotPath, gotBody string
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit, gotPath = true, r.URL.Path
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "data": map[string]any{"posts": []any{
				map[string]any{
					"post":   map[string]any{"id": 100, "thread_id": 7, "author_id": 55, "content_raw": "hi", "status": 0},
					"thread": map[string]any{"thread_id": 7, "anchor_kind": 1, "anchor_id": "42"},
				},
			}},
		})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	out, err := c.ResolvePosts(context.Background(), []int64{100, 200})
	if err != nil {
		t.Fatalf("ResolvePosts: %v", err)
	}
	if gotPath != "/posts/resolve" || !strings.Contains(gotBody, `"ids":[100,200]`) {
		t.Errorf("path = %q body = %q", gotPath, gotBody)
	}
	if len(out.Posts) != 1 || out.Posts[0].Post.ID != 100 {
		t.Errorf("decoded = %+v", out)
	}
	hit = false
	if res, err := c.ResolvePosts(context.Background(), nil); err != nil || len(res.Posts) != 0 || hit {
		t.Errorf("empty ResolvePosts hit=%v res=%+v err=%v", hit, res, err)
	}
}

// TestListPostsQuery proves after/limit ride as query params and empty values
// are omitted (community defaults then apply).
func TestListPostsQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"posts": []any{}}})
	}))
	defer srv.Close()

	if _, err := newTestClient(srv.URL).ListPosts(context.Background(), 7, "50", "30"); err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	for _, want := range []string{"after=50", "limit=30"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}
