package communityclient

// Wire types mirror kun-galgame-infra/internal/platform/community/dto +
// docs/community/openapi.yaml. Field names and JSON tags match the contract
// verbatim so the envelope decodes directly. Optional community fields (*int64
// on the wire) are modeled as plain int64 with 0 = absent, matching the letmoe
// consumer — the BFF treats 0 as "no pointer".

// Anchor kinds (ResolveCommentsRequest.anchor_kind).
const (
	AnchorBoard         = 0 // board topic (kind=0 threads)
	AnchorSiteGame      = 1 // a site's local game_id (game comments)
	AnchorSiteResource  = 2 // a site's local resource_id
	AnchorCatalogWork   = 3
	AnchorCatalogPerson = 4
)

// Thread kinds (ThreadView.kind).
const (
	KindTopic    = 0
	KindComments = 1
	KindFeedback = 2
)

// Content rating (ResolveCommentsRequest.content_rating / *.content_rating).
const (
	RatingAll = 0
	RatingR15 = 1
	RatingR18 = 2
)

// Reaction kinds (ReactionToggleRequest.kind).
const ReactionLike = 0

// Post status (PostView.status).
const (
	PostVisible = 0
	PostHeld    = 1 // TL0 first-post hold — created hidden, enqueued for review
	PostDeleted = 2 // author self-delete / mod reject — tombstone, post_number kept
)

// Boost levels (SetBoostRequest.boost).
const (
	BoostNone    = 0
	BoostVeteran = 1
	BoostCreator = 2
	BoostStaff   = 3
)

// Flag reasons (FlagRequest.reason).
const (
	FlagReasonSpam         = 0
	FlagReasonAbuse        = 1
	FlagReasonOffTopic     = 2
	FlagReasonOther        = 3
	FlagReasonNsfwMislabel = 4
)

// PostView is a single post (opening post or reply). Reaction counts are NOT
// part of the community read model — kungal assembles like_count from its local
// galgame_post_like table (charter ruling 8).
type PostView struct {
	ID            int64  `json:"id"`
	ThreadID      int64  `json:"thread_id"`
	PostNumber    int32  `json:"post_number"`
	AuthorID      int64  `json:"author_id"`
	ContentRaw    string `json:"content_raw"`
	ContentHTML   string `json:"content_html"`
	ContentRating int32  `json:"content_rating"`
	Status        int32  `json:"status"`
	CreatedAt     string `json:"created_at"`
	EditedAt      string `json:"edited_at"`
	// EditedByModerator reports whether the LATEST edit was a mod-actor edit
	// (docs/proj/17 decision 3) — drives the "已编辑（管理）" label.
	EditedByModerator bool `json:"edited_by_moderator"`
	// The primitive completes root_post_id/target_user_id from the parent row
	// (the BFF passes only reply_to_post_id); 0 = absent (a top-level comment).
	ReplyToPostID int64 `json:"reply_to_post_id"`
	RootPostID    int64 `json:"root_post_id"`
	TargetUserID  int64 `json:"target_user_id"`
}

// ThreadView is a thread's metadata (game comments thread, kind=1).
type ThreadView struct {
	ID                int64  `json:"id"`
	Site              string `json:"site"`
	Kind              int32  `json:"kind"`
	AnchorKind        int32  `json:"anchor_kind"`
	AnchorID          string `json:"anchor_id"`
	ContentRating     int32  `json:"content_rating"`
	Status            int32  `json:"status"`
	PostsCount        int32  `json:"posts_count"`
	ParticipantsCount int32  `json:"participants_count"`
	HighestPostNumber int32  `json:"highest_post_number"`
	CreatedBy         int64  `json:"created_by"`
	CreatedAt         string `json:"created_at"`
	LastPostedAt      string `json:"last_posted_at"`
}

// ThreadWithPosts is a thread plus a page of posts (resolveComments / listPosts
// share the shape; the primitive computes next_cursor server-side — empty = last
// page).
type ThreadWithPosts struct {
	Thread     ThreadView `json:"thread"`
	Posts      []PostView `json:"posts"`
	NextCursor string     `json:"next_cursor"`
}

// PostListResponse is a keyset page of posts (listPosts).
type PostListResponse struct {
	Posts      []PostView `json:"posts"`
	NextCursor string     `json:"next_cursor"`
}

// PostThreadContext is the minimal render context of a post's thread, projected
// into the cross-thread by-author / resolve views so the consumer can render a
// jump link (title for the label, anchor for the URL) without a per-post round
// trip. ThreadID repeats the embedded post's thread_id. Title is empty for a
// comments thread (null on the wire), set for topic/feedback threads.
type PostThreadContext struct {
	ThreadID   int64  `json:"thread_id"`
	Title      string `json:"title"`
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
}

// AuthorPostView is one post plus its thread render context — the unit of the
// by-author profile list and the by-id resolve response. PostView is reused
// verbatim (nested under post); no new field semantics.
type AuthorPostView struct {
	Post   PostView          `json:"post"`
	Thread PostThreadContext `json:"thread"`
}

// AuthorPostsResponse is a keyset page of an author's visible posts (descending
// post id). NextCursor = the post id to pass as `after` for the next (older)
// page; empty = last page.
type AuthorPostsResponse struct {
	Posts      []AuthorPostView `json:"posts"`
	NextCursor string           `json:"next_cursor"`
}

// PostsResolveResponse is the batch by-id hydration result: the VISIBLE posts
// among the requested ids, in request order (post-dedupe). Hidden/deleted/
// cross-site/unknown ids are SILENTLY absent — the caller diffs the request set
// against the returned set to render placeholders (no per-id status leaks).
type PostsResolveResponse struct {
	Posts []AuthorPostView `json:"posts"`
}

// AuthorStat is one author's visible-post count in this site.
type AuthorStat struct {
	AuthorID     int64 `json:"author_id"`
	VisiblePosts int64 `json:"visible_posts"`
}

// AuthorStatsResponse is the batch visible-post count — exactly one entry per
// requested id (unknown ids report 0), in request order.
type AuthorStatsResponse struct {
	Stats []AuthorStat `json:"stats"`
}

// PurgeResult reports the effect of a compliance purge on the community side.
// A second purge of the same author on the same site reports zero for both
// (idempotent).
type PurgeResult struct {
	PostsPurged      int64 `json:"posts_purged"`
	ReactionsDeleted int64 `json:"reactions_deleted"`
}

// TrustView is a user's trust row (setBoost returns it).
type TrustView struct {
	UserID                  int64 `json:"user_id"`
	Level                   int32 `json:"level"`
	FirstPostsHeldRemaining int32 `json:"first_posts_held_remaining"`
	GrantedBoost            int32 `json:"granted_boost"`
}

// --- request bodies (the acting user identity is BFF-supplied) ---

type ResolveCommentsRequest struct {
	AnchorKind    int32  `json:"anchor_kind"`
	AnchorID      string `json:"anchor_id"`
	ContentRating int32  `json:"content_rating"`
}

// PostsResolveRequest hydrates a batch of posts by id (the like-tab read: kungal
// stores only community post ids locally and needs their content back). ids is
// capped at 100 server-side (422 over) and de-duplicated preserving first-seen
// order; only visible posts return.
type PostsResolveRequest struct {
	IDs []int64 `json:"ids"`
}

// ReplyRequest appends a post. For a REPLY the BFF passes only reply_to_post_id
// and the primitive derives root_post_id (the sub-thread's top ancestor) and
// target_user_id (the parent's author) from the parent row (verified against
// service/post.go Reply). TargetUserID is an explicit override the primitive
// accepts (json:"target_user_id,omitempty") — the galgame comment path never
// sets it (0 = omitted, so its behavior is unchanged), but the FLAT rating
// comment area carries an explicit "A → B" target with no parent post, so it
// passes TargetUserID through directly (charter ruling 19).
type ReplyRequest struct {
	AuthorID      int64  `json:"author_id"`
	Body          string `json:"body"`
	ReplyToPostID int64  `json:"reply_to_post_id,omitempty"`
	TargetUserID  int64  `json:"target_user_id,omitempty"`
}

// EditPostRequest edits a post (re-cooked + edited_at stamped). author_id must
// match the post's author unless as_moderator declares the mod-actor variant —
// the BFF vouches the actor is a kungal moderator (role.CanModerate) and the
// primitive audit-logs the action (docs/proj/17 decision 3).
type EditPostRequest struct {
	AuthorID    int64  `json:"author_id"`
	Body        string `json:"body"`
	AsModerator bool   `json:"as_moderator,omitempty"`
}

type ReactionToggleRequest struct {
	UserID int64 `json:"user_id"`
	Kind   int32 `json:"kind"`
}

// ReactionToggleResult reports the new reaction state plus the post's context
// (author + thread/anchor) — the recipient and jump target of the like
// notification, resolved by the reaction flow anyway (docs/proj/17). Added=true
// means the reaction was just added, false means it was just removed.
type ReactionToggleResult struct {
	Added      bool   `json:"added"`
	AuthorID   int64  `json:"author_id"`
	ThreadID   int64  `json:"thread_id"`
	AnchorKind int32  `json:"anchor_kind"`
	AnchorID   string `json:"anchor_id"`
}

// FlagRequest reports a post (reputation-weighted; may auto-hide + enqueue).
type FlagRequest struct {
	FlaggerID int64  `json:"flagger_id"`
	Reason    int32  `json:"reason"`
	Note      string `json:"note,omitempty"`
}

// SetBoostRequest declares a starter boost (veteran/creator/staff) for a user. A
// boost only ever raises the floor; the server is idempotent.
type SetBoostRequest struct {
	UserID int64 `json:"user_id"`
	Boost  int32 `json:"boost"`
}
