package catalogclient

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// The claim-lifecycle face (infra wave 155 W2/W3 + 157): the eight semantic
// actions, the transition feed downstream inboxes are built from, and the
// per-user claim list.
//
// This is the registry-native replacement for the wiki's submission surface.
// Two vocabularies retire with it and are NOT re-declared here:
//
//   - the wiki `status` integers (0 published / 1 banned / 2 VNDB draft /
//     3 pending / 4 declined) become the five claim states below, which is a
//     re-shape and not a rename: status 2 and status 3 both projected onto
//     `draft`, and `hidden` is a state the wiki expressed as a deleted row;
//   - the wiki message `type` words (approved / declined / banned / unbanned)
//     become (from_state, to_state) PAIRS. A consumer that needs "was this an
//     approval" asks the transition, not a label — which is what lets the same
//     feed carry transitions the wiki had no word for.
//
// Every call rides the same Basic-authed S2S channel and envelope transport as
// the edit face (editDo), so a 409 keeps its body: an illegal transition
// answers with the claim's CURRENT state, which is the whole reason the action
// endpoints exist instead of a claim_state patch.

// Claim states — the public claim vocabulary (`claimed_by.state`). Eternal
// wire values.
const (
	ClaimStateNone     = "none"
	ClaimStateLive     = "live"
	ClaimStateDraft    = "draft"
	ClaimStatePending  = "pending"
	ClaimStateDeclined = "declined"
	ClaimStateHidden   = "hidden"
)

// Claim actions — the eight semantic moves. The first four are the owning
// site's, the last four a reviewer's (infra requires catalog.claim.review,
// which wave 157 granted moderator and up so the wiki's moderation staffing
// carries over unchanged).
const (
	ClaimActionClaim    = "claim"
	ClaimActionSubmit   = "submit"
	ClaimActionPublish  = "publish"
	ClaimActionWithdraw = "withdraw"
	ClaimActionApprove  = "approve"
	ClaimActionDecline  = "decline"
	ClaimActionBan      = "ban"
	ClaimActionUnban    = "unban"
)

// ClaimActionRequest is the body of one lifecycle action. Site is the acting
// tenant and must equal this client's catalog_site binding for the four owner
// actions; review actions ignore it (a reviewer acts across tenants and the
// event still records the claim's own site).
type ClaimActionRequest struct {
	Site  string    `json:"site,omitempty"`
	Actor EditActor `json:"actor"`
	// ProductWorkID is read by `claim` alone — the product-side id the registry
	// row starts pointing at — and ignored by every other action.
	ProductWorkID int64 `json:"product_work_id,omitempty"`
	// Reason is recorded on the event. REQUIRED by decline: a refusal the
	// submitter cannot act on is the failure mode the field exists to prevent.
	Reason string `json:"reason,omitempty"`
}

// ClaimActionResult is what the transition did.
type ClaimActionResult struct {
	WorkID  int64   `json:"work_id"`
	From    *string `json:"from_state"`
	To      string  `json:"to_state"`
	EventID int64   `json:"event_id"`
}

// ActOnClaim performs one lifecycle action on a registry work.
//
// An illegal transition surfaces as an *EditAPIError with status 409 whose
// message names the current state — callers render it rather than retrying,
// because the losing side of a race has already had its answer.
func (c *Client) ActOnClaim(ctx context.Context, workID int64, action string, req ClaimActionRequest) (*ClaimActionResult, error) {
	return editPost[ClaimActionResult](ctx,
		c, "/api/v1/catalog/works/"+strconv.FormatInt(workID, 10)+"/claim-actions/"+action, req)
}

// WorkSubmitRequest mints a registry work straight into `pending` — the
// registry-native replacement for the wiki's "submit a new galgame" write.
//
// Fields is keyed by the SAME field keys the editing face registers for
// catalog.work, so one schema drives both the submission form and every later
// edit of the same entry. Two keys the matrix does not have and this face
// therefore refuses: a work-level release date (send Released; it becomes one
// curated release row) and a status (lifecycle is claim_state, moved only by
// the semantic actions). Covers and screenshots are not accepted either — the
// bytes are uploaded first and attached as a later edit.
//
// ProductWorkID is OMITTED by kungal, and that is the whole id story: the
// registry issues the identity and the claim adopts the minted work's own id,
// which the response returns. One allocator instead of two.
//
// The alternative — the forum handing out ids from its own sequence — is what
// this replaces, and it was a standing hazard rather than a mere preference:
// two sequences advancing independently over one shared key space stay correct
// only for as long as somebody keeps reseeding the follower, and the failure is
// silent (a collision reads as "your submission already exists"). Adopting a
// primary key is unique by construction and has nothing to keep in step.
type WorkSubmitRequest struct {
	Site  string    `json:"site"`
	Actor EditActor `json:"actor"`
	// ProductWorkID anchors the claim at an id the product allocated. Omit it
	// (0) to let the registry issue one; `omitempty` is what carries that.
	ProductWorkID int64           `json:"product_work_id,omitempty"`
	Fields        map[string]any  `json:"fields"`
	Released      *WorkSubmitDate `json:"released,omitempty"`
}

// WorkSubmitDate is the fuzzy submitted date. The nullable tail IS the
// precision — {Y:2019} means "sometime in 2019" — so there is no separate
// precision enum and an omitted date means TBA.
type WorkSubmitDate struct {
	Y int16 `json:"y"`
	M int16 `json:"m,omitempty"`
	D int16 `json:"d,omitempty"`
}

// WorkSubmitResult is the minted identity plus its birth event.
type WorkSubmitResult struct {
	WorkID int64 `json:"work_id"`
	// ProductWorkID is the id the claim ended up anchored at. When the request
	// omitted one it is the registry-issued identity — and therefore the gid
	// kungal must use from here on. Always read this rather than assuming it
	// equals WorkID: it is the field that stays correct either way.
	ProductWorkID int64  `json:"product_work_id"`
	ClaimState    string `json:"claim_state"`
	EventID       int64  `json:"event_id"`
	ReleaseID     int64  `json:"release_id,omitempty"`
}

// SubmitWork files a new submission. Idempotency is the claim's own unique key
// (medium, site, product_work_id): a retried wizard gets a 409 naming the work
// it already created and its current state, and never mints a second identity.
func (c *Client) SubmitWork(ctx context.Context, req WorkSubmitRequest) (*WorkSubmitResult, error) {
	return editPost[WorkSubmitResult](ctx, c, "/api/v1/catalog/works/submit", req)
}

// ClaimEventFeedItem is one claim transition.
//
// FromState is null exactly once per claim — the transition that created it —
// so a consumer can recognise a birth without a second read. ProductWorkID is
// the claim's CURRENT product-side id (a snapshot taken when the page was
// served, not the value at event time); for a kungal claim that is the gid.
type ClaimEventFeedItem struct {
	ID            int64     `json:"id"`
	WorkID        int64     `json:"work_id"`
	FromState     *string   `json:"from_state"`
	ToState       string    `json:"to_state"`
	ActorUID      int64     `json:"actor_uid"`
	Reason        *string   `json:"reason"`
	Site          string    `json:"site"`
	ProductWorkID *int64    `json:"product_work_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// ClaimEventFeedPage is one page of the transition feed. NextSince echoes the
// request's cursor on an empty page, so a consumer that stores it
// unconditionally never rewinds.
type ClaimEventFeedPage struct {
	Items     []ClaimEventFeedItem `json:"items"`
	NextSince int64                `json:"next_since"`
}

// ClaimEventsSince reads one page of the transition feed after the exclusive
// cursor `since`, optionally narrowed to one tenant. Ascending by id, no
// has_more flag: a page shorter than the limit is the tail.
func (c *Client) ClaimEventsSince(ctx context.Context, since int64, limit int, site string) (*ClaimEventFeedPage, error) {
	q := url.Values{
		"since": {strconv.FormatInt(since, 10)},
		"limit": {strconv.Itoa(limit)},
	}
	if site != "" {
		q.Set("site", site)
	}
	return editGetQuery[ClaimEventFeedPage](ctx, c, "/api/v1/catalog/claim-events/feed", q)
}

// UserClaimItem is one work a user has moved through its lifecycle — the
// registry's answer to "my submissions".
//
// The Last* block is the work's LATEST transition BY ANYONE, not by this user.
// That is the point of it: what a submitter needs on their own submission is
// the reviewer's verdict and note, which is by definition an event they did
// not cause.
type UserClaimItem struct {
	WorkID        int64  `json:"work_id"`
	DisplayName   string `json:"display_name"`
	Site          string `json:"site"`
	ProductWorkID *int64 `json:"product_work_id"`
	ClaimState    string `json:"claim_state"`

	LastEventID   int64     `json:"last_event_id"`
	LastFromState *string   `json:"last_from_state"`
	LastToState   string    `json:"last_to_state"`
	LastReason    *string   `json:"last_reason"`
	LastActorUID  int64     `json:"last_actor_uid"`
	LastEventAt   time.Time `json:"last_event_at"`

	// FirstActedAt is when this user first touched the claim — "submitted on"
	// for the common case.
	FirstActedAt time.Time `json:"first_acted_at"`
	// ActedCount is how many transitions this user caused on this work.
	ActedCount int `json:"acted_count"`
}

// UserClaimPage is one DESCENDING cursor page.
//
// Total is the count under the SAME filter, independent of the cursor — which
// is what makes a separate per-user stats endpoint unnecessary: "how many works
// has this user published" is this call with claim_state=live and limit=1.
type UserClaimPage struct {
	Items []UserClaimItem `json:"items"`
	// NextBefore is the cursor for the following page; 0 = no more rows.
	NextBefore int64 `json:"next_before"`
	Total      int64 `json:"total"`
}

// UserClaimFilter is one page request against the per-user face.
type UserClaimFilter struct {
	// Site restricts to one tenant (empty = every site the user acted on).
	Site string
	// ClaimStates is the public vocabulary filter; empty = every state.
	ClaimStates []string
	// Before is the exclusive cursor (the previous page's next_before);
	// 0 = the first page.
	Before int64
	Limit  int
}

// UserClaims lists the works a user has acted on, newest activity first.
func (c *Client) UserClaims(ctx context.Context, uid int64, f UserClaimFilter) (*UserClaimPage, error) {
	q := url.Values{}
	if f.Site != "" {
		q.Set("site", f.Site)
	}
	if len(f.ClaimStates) > 0 {
		q.Set("claim_state", joinStates(f.ClaimStates))
	}
	if f.Before > 0 {
		q.Set("before", strconv.FormatInt(f.Before, 10))
	}
	if f.Limit > 0 {
		q.Set("limit", strconv.Itoa(f.Limit))
	}
	return editGetQuery[UserClaimPage](ctx,
		c, "/api/v1/catalog/users/"+strconv.FormatInt(uid, 10)+"/claims", q)
}

func joinStates(states []string) string {
	out := ""
	for i, s := range states {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}

// editGetQuery is editGet with a query string. It keeps the envelope's
// business code so a 400 on a bad claim_state token reaches the caller as an
// actionable message rather than a bare 503.
func editGetQuery[T any](ctx context.Context, c *Client, path string, q url.Values) (*T, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return editGet[T](ctx, c, path)
}
