package communityclient

import (
	"context"
	"net/http"
)

// ResolveComments gets-or-creates the single comments thread for an anchor and
// returns its first page of posts (the read-first-screen; idempotent). For
// galgame comments: anchor_kind=site_game, anchor_id=galgame id text,
// content_rating=all (kungal's comment read face is gated by the site page).
func (c *Client) ResolveComments(ctx context.Context, req ResolveCommentsRequest) (*ThreadWithPosts, error) {
	var out ThreadWithPosts
	err := c.do(ctx, http.MethodPost, "/comments/resolve", req, &out)
	return &out, err
}

// ListPosts returns a thread's posts after a post_number, ascending (keyset).
// after/limit empty → community defaults (limit 50, from the top). The primitive
// clamps limit to 100 server-side.
func (c *Client) ListPosts(ctx context.Context, threadID int64, after, limit string) (*PostListResponse, error) {
	var out PostListResponse
	q := query(map[string]string{"after": after, "limit": limit})
	err := c.do(ctx, http.MethodGet, "/threads/"+itoa(threadID)+"/posts"+q, nil, &out)
	return &out, err
}

// Reply appends a post to a thread and returns the created post view.
func (c *Client) Reply(ctx context.Context, threadID int64, req ReplyRequest) (*PostView, error) {
	var out struct {
		Post PostView `json:"post"`
	}
	err := c.do(ctx, http.MethodPost, "/threads/"+itoa(threadID)+"/posts", req, &out)
	return &out.Post, err
}

// EditPost edits a post's body (author, or a moderator via as_moderator;
// re-cooked + edited_at stamped) and returns the updated post view.
func (c *Client) EditPost(ctx context.Context, postID int64, req EditPostRequest) (*PostView, error) {
	var out struct {
		Post PostView `json:"post"`
	}
	err := c.do(ctx, http.MethodPatch, "/posts/"+itoa(postID), req, &out)
	return &out.Post, err
}

// DeletePost tombstones a post (post_number preserved, replies kept). The acting
// user rides as a query param (?author_id=) — the community DELETE takes no body.
// asModerator declares the mod-actor variant (docs/proj/17 decision 3): the
// author match is skipped and the primitive audit-logs the action.
func (c *Client) DeletePost(ctx context.Context, postID, authorID int64, asModerator bool) error {
	q := map[string]string{"author_id": itoa(authorID)}
	if asModerator {
		q["as_moderator"] = "true"
	}
	return c.do(ctx, http.MethodDelete, "/posts/"+itoa(postID)+query(q), nil, nil)
}

// ToggleReaction toggles a reaction on a post; result.Added reports the new
// state, and the result carries the post's author + thread/anchor for the like
// notification fan-out.
func (c *Client) ToggleReaction(ctx context.Context, postID int64, req ReactionToggleRequest) (*ReactionToggleResult, error) {
	var out ReactionToggleResult
	err := c.do(ctx, http.MethodPost, "/posts/"+itoa(postID)+"/reaction", req, &out)
	return &out, err
}

// SubmitFlag reports a post (reputation-weighted; may auto-hide + enqueue into
// the moderation queue).
func (c *Client) SubmitFlag(ctx context.Context, postID int64, req FlagRequest) error {
	return c.do(ctx, http.MethodPost, "/posts/"+itoa(postID)+"/flag", req, nil)
}

// Boost declares a starter boost (veteran/creator/staff) for a user. A boost
// only ever raises the floor; the server is idempotent.
func (c *Client) Boost(ctx context.Context, req SetBoostRequest) (*TrustView, error) {
	var out TrustView
	err := c.do(ctx, http.MethodPost, "/trust/boost", req, &out)
	return &out, err
}

// --- author read / purge face (charter step 06a; infra step 10 + 11) ---

// AuthorPosts returns a keyset page of an author's visible posts across threads
// (descending post id), each with its thread render context — the profile
// comment tab read. after is the post-id cursor ("" = newest page); limit ≤0
// defers to the server default (20, cap 100); anchorKind ≥0 filters to one kind
// (e.g. AnchorSiteGame), <0 = all kinds.
func (c *Client) AuthorPosts(ctx context.Context, authorID int64, after string, limit, anchorKind int) (*AuthorPostsResponse, error) {
	var out AuthorPostsResponse
	q := map[string]string{"after": after}
	if limit > 0 {
		q["limit"] = itoa(int64(limit))
	}
	if anchorKind >= 0 {
		q["anchor_kind"] = itoa(int64(anchorKind))
	}
	err := c.do(ctx, http.MethodGet, "/authors/"+itoa(authorID)+"/posts"+query(q), nil, &out)
	return &out, err
}

// AuthorStats returns the per-author visible-post count for a batch of user ids
// (≤100; the server 422s over) — the profile / hover-card stat. Exactly one row
// per requested id (unknown = 0), in request order. An empty id list short-
// circuits to an empty result with no network hit.
func (c *Client) AuthorStats(ctx context.Context, ids []int64) (*AuthorStatsResponse, error) {
	if len(ids) == 0 {
		return &AuthorStatsResponse{Stats: []AuthorStat{}}, nil
	}
	var out AuthorStatsResponse
	err := c.do(ctx, http.MethodGet, "/authors/stats"+query(map[string]string{"ids": joinInt64(ids)}), nil, &out)
	return &out, err
}

// AuthorPurge tombstones + scrubs every post the author left in this site and
// deletes their reaction rows (compliance purge). No request body; idempotent
// (a second run reports {0,0}).
func (c *Client) AuthorPurge(ctx context.Context, authorID int64) (*PurgeResult, error) {
	var out PurgeResult
	err := c.do(ctx, http.MethodPost, "/authors/"+itoa(authorID)+"/purge", nil, &out)
	return &out, err
}

// ResolvePosts hydrates a batch of posts by id (≤100). Only visible posts come
// back, in request order; hidden/deleted/cross-site/unknown ids are silently
// absent (the caller renders placeholders for the difference). An empty id list
// short-circuits to an empty result with no network hit.
func (c *Client) ResolvePosts(ctx context.Context, ids []int64) (*PostsResolveResponse, error) {
	if len(ids) == 0 {
		return &PostsResolveResponse{Posts: []AuthorPostView{}}, nil
	}
	var out PostsResolveResponse
	err := c.do(ctx, http.MethodPost, "/posts/resolve", PostsResolveRequest{IDs: ids}, &out)
	return &out, err
}
