// Package userclient is the HTTP SDK kungal uses to look up user identity
// from the OAuth server. Identity (name / avatar / bio / status / roles) is
// owned by OAuth post-migration; kungal stores only foreign keys (user_id)
// in business rows. Mapper layers call Users(ctx, ids) to enrich rows for
// rendering.
//
// Features:
//   - HTTP Basic Auth with OAuth client_id:client_secret (per OAuth doc § 10)
//   - In-memory TTL cache for hot users (default 10 min)
//   - Negative cache for not_found ids (default 1 min) to avoid repeat misses
//   - golang.org/x/sync/singleflight for in-flight dedup
//   - Auto-shard >100-id requests into 100-each chunks
//   - Ban-aware: status != 0 users get a placeholder name to keep render
//     paths from crashing while the caller decides whether to hide the row
package userclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"kun-galgame-api/pkg/imageclient"
	"kun-galgame-api/pkg/role"

	"golang.org/x/sync/singleflight"
)

// Config configures a Client. Caches are kept per Client instance — make it
// a singleton for cache to be effective.
type Config struct {
	BaseURL       string        // OAuth server, e.g. http://127.0.0.1:9277/api/v1
	ClientID      string        // OAuth Client (e.g. "kungal-backend")
	ClientSecret  string        // OAuth Client secret
	ImageCDNBase  string        // image_service CDN base, for resolving avatar_image_hash → URL
	CacheTTL      time.Duration // hot cache TTL — defaults to 10 min
	NegCacheTTL   time.Duration // negative (not_found) TTL — defaults to 1 min
	HTTPTimeout   time.Duration // single-request timeout — defaults to 5 sec
	BatchPageSize int           // ids/request — defaults to 100 (OAuth max)
}

// User mirrors /users/batch's per-user payload.
type User struct {
	ID              int      `json:"id"`
	UUID            string   `json:"uuid"`
	Name            string   `json:"name"`
	Avatar          string   `json:"avatar"`
	AvatarImageHash string   `json:"avatar_image_hash"`
	Bio             string   `json:"bio"`
	Status          int      `json:"status"`
	Roles           []string `json:"roles"`
	// SiteRoles is this user's site-scoped roles for OUR client's site (contract
	// docs/oauth/12-site-roles.md; the /users/batch brief is scoped by the
	// requesting client's site). Folded into Roles in fetchBatch so consumers
	// read ONE effective set — kept here so a future site-vs-global distinction
	// (e.g. a 「本站版主」 badge) can recover it (global = Roles \ SiteRoles).
	SiteRoles []string `json:"site_roles"`
	// CreatedAt is the user's OAuth registration time (UTC RFC3339). This is
	// the authoritative "join date" — render it from here, NOT from a local
	// kungal_user_state.created (which marks first-seen-on-kungal: blank for
	// users who never logged into the forum, wrong for those whose first
	// login lagged registration).
	CreatedAt string `json:"created_at"`
}

// Client is safe for concurrent use across goroutines.
type Client struct {
	cfg    Config
	http   *http.Client
	authHd string

	imageCDNBase string

	mu       sync.RWMutex
	hot      map[int]cacheEntry
	miss     map[int]time.Time
	sfGroup  singleflight.Group
	negTTL   time.Duration
	hotTTL   time.Duration
	pageSize int
}

type cacheEntry struct {
	user   User
	expire time.Time
}

// New returns a configured Client. Defaults fill missing fields.
func New(cfg Config) *Client {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 10 * time.Minute
	}
	if cfg.NegCacheTTL == 0 {
		cfg.NegCacheTTL = 1 * time.Minute
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.BatchPageSize == 0 || cfg.BatchPageSize > 100 {
		cfg.BatchPageSize = 100
	}
	return &Client{
		cfg:          cfg,
		http:         &http.Client{Timeout: cfg.HTTPTimeout},
		authHd:       "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret)),
		imageCDNBase: strings.TrimRight(cfg.ImageCDNBase, "/"),
		hot:          map[int]cacheEntry{},
		miss:         map[int]time.Time{},
		hotTTL:       cfg.CacheTTL,
		negTTL:       cfg.NegCacheTTL,
		pageSize:     cfg.BatchPageSize,
	}
}

// resolveAvatarURL maps a user's avatar to a render-ready URL. The new avatar
// pipeline (POST /auth/me/avatar) writes only `avatar_image_hash` and leaves the
// legacy `avatar` URL empty, so prefer the content hash → image_service URL and
// fall back to the legacy `avatar` (old users not yet migrated). Without this,
// every user who set a new avatar renders blank — OAuth returns an empty
// `avatar` and nothing resolved the hash.
func (c *Client) resolveAvatarURL(u User) string {
	if c.imageCDNBase != "" && u.AvatarImageHash != "" {
		if url := imageclient.MainURL(c.imageCDNBase, u.AvatarImageHash, "webp"); url != "" {
			return url
		}
	}
	return u.Avatar
}

// envelope is the OAuth API envelope shape `{code, message, data}`.
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// batchData is the data field of /users/batch.
type batchData struct {
	Users    []User `json:"users"`
	NotFound []int  `json:"not_found"`
}

// searchData is the data field of /users/search.
//
// OAuth returns `{users: [...]}` with no total — /users/search is a
// relevance-ranked top-N suggestion endpoint, not a paginated list (see
// docs/migration/user/08-downstream-integration.md §3.2). Earlier this
// struct decoded `{items, total}` which silently produced an empty result.
type searchData struct {
	Users []User `json:"users"`
}

// Users returns a map of id -> User. Missing IDs are absent (not nil entries).
// Order is irrelevant; caller looks up by id.
//
// The function transparently:
//   - serves from cache when fresh
//   - skips IDs in negative cache
//   - dedups concurrent requests for the same id via singleflight
//   - shards >100-id requests
//
// Returns the union of all known users; never returns an error solely because
// some ids are missing. A returned error means the OAuth call itself failed
// (network / 5xx / auth). On error, the partial cache hits are still in the map.
func (c *Client) Users(ctx context.Context, ids []int) (map[int]User, error) {
	out := map[int]User{}
	if len(ids) == 0 {
		return out, nil
	}

	// Dedup ids and pick out already-cached / known-missing.
	now := time.Now()
	seen := map[int]struct{}{}
	missing := make([]int, 0, len(ids))

	c.mu.RLock()
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if e, ok := c.hot[id]; ok && now.Before(e.expire) {
			out[id] = e.user
			continue
		}
		if t, ok := c.miss[id]; ok && now.Before(t) {
			continue // negative cache hit; skip
		}
		missing = append(missing, id)
	}
	c.mu.RUnlock()

	if len(missing) == 0 {
		return out, nil
	}

	// Shard remaining missing ids into pageSize chunks; fetch each with
	// singleflight keyed by the joined-id string so concurrent callers
	// asking for overlapping ids share work.
	for start := 0; start < len(missing); start += c.pageSize {
		end := start + c.pageSize
		if end > len(missing) {
			end = len(missing)
		}
		shard := missing[start:end]

		key := joinIntsForKey(shard)
		raw, err, _ := c.sfGroup.Do(key, func() (any, error) {
			return c.fetchBatch(ctx, shard)
		})
		if err != nil {
			return out, err
		}
		bd := raw.(batchData)
		c.cacheStore(bd, now)
		for _, u := range bd.Users {
			out[u.ID] = u
		}
	}
	return out, nil
}

// User is the single-id convenience. Returns (zero, false) for unknown ids.
func (c *Client) User(ctx context.Context, id int) (User, bool, error) {
	m, err := c.Users(ctx, []int{id})
	if err != nil {
		return User{}, false, err
	}
	u, ok := m[id]
	return u, ok, nil
}

// SearchUsers proxies OAuth /users/search. Results are not cached (search
// queries are too varied to cache effectively).
//
// OAuth's /users/search is a top-N relevance-ranked suggestion endpoint, not a
// paginated list — it only accepts `q` + `limit` and returns up to 50 users
// without a total. The caller should treat len(result) as the total for UI
// purposes (mention autocomplete / search-as-you-type rather than infinite
// scroll). `limit` is clamped to OAuth's 50 max.
func (c *Client) SearchUsers(ctx context.Context, q string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	endpoint := c.cfg.BaseURL + "/users/search?" + url.Values{
		"q":     {q},
		"limit": {strconv.Itoa(limit)},
	}.Encode()
	var sd searchData
	if err := c.do(ctx, "GET", endpoint, &sd); err != nil {
		return nil, err
	}
	for i := range sd.Users {
		sd.Users[i].Avatar = c.resolveAvatarURL(sd.Users[i])
	}
	// Opportunistically warm the hot cache so a subsequent batch hit is free.
	now := time.Now()
	c.mu.Lock()
	for _, u := range sd.Users {
		c.hot[u.ID] = cacheEntry{user: u, expire: now.Add(c.hotTTL)}
	}
	c.mu.Unlock()
	return sd.Users, nil
}

// Placeholder builds a render-safe stub for users that are not_found or
// banned (status != 0). Mappers can call this so render paths stay
// non-nil even when an OAuth row is missing.
//
// id may be 0 to denote "unknown user" entirely.
func Placeholder(id int) User {
	return User{ID: id, Name: "已注销用户", Avatar: ""}
}

// fetchBatch makes a single /users/batch call with the given shard.
func (c *Client) fetchBatch(ctx context.Context, ids []int) (batchData, error) {
	endpoint := c.cfg.BaseURL + "/users/batch?ids=" + joinInts(ids, ",")
	var bd batchData
	if err := c.do(ctx, "GET", endpoint, &bd); err != nil {
		return bd, err
	}
	// Resolve avatar_image_hash → URL once here so the resolved value is what
	// gets cached and handed to every consumer (top bar, comment lists, …).
	for i := range bd.Users {
		bd.Users[i].Avatar = c.resolveAvatarURL(bd.Users[i])
		// Fold this-site's site_roles into Roles so every consumer reads ONE
		// EFFECTIVE set (global ∪ site, 12-site-roles §5.1) — a site moderator is
		// recognised here (purge protection, badge, isCreator). The batch is
		// site-scoped (§2.3), so this is identity resolution, not capability
		// policy; site_roles never carries admin/ren (§5.3), so it can't
		// over-privilege.
		bd.Users[i].Roles = role.Union(bd.Users[i].Roles, bd.Users[i].SiteRoles)
	}
	return bd, nil
}

// sendEnvelope is the single request path behind do/doJSON/doJSONWithToken:
// build the request, run it, decode the {code,message,data} envelope, and
// unmarshal data into v. `auth` is the full Authorization header value (Basic
// client creds or a user Bearer). A nil body sends no request body (GET).
//
// A non-zero envelope code returns *OAuthError so callers can branch on
// business codes (e.g. 16004 idempotency conflict). A nil/absent data field
// (OAuth omits `data` for "nothing to return", e.g. a never-applied creator on
// GET /creator/applications/me) is a no-op — leaving v untouched instead of
// failing unmarshal with "unexpected end of JSON input".
func (c *Client) sendEnvelope(ctx context.Context, method, endpoint, auth string, body, v any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("userclient: %w", err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("userclient: decode envelope: %w", err)
	}
	if env.Code != 0 {
		return &OAuthError{Code: env.Code, Message: env.Message}
	}
	if v == nil || len(env.Data) == 0 {
		return nil
	}
	return json.Unmarshal(env.Data, v)
}

// do is a bodiless GET/DELETE authenticated with the client-credentials Basic
// header. v should be a pointer.
func (c *Client) do(ctx context.Context, method, endpoint string, v any) error {
	return c.sendEnvelope(ctx, method, endpoint, c.authHd, nil, v)
}

// doJSON is do() with a JSON request body (POST/PUT), same Basic auth.
func (c *Client) doJSON(ctx context.Context, method, endpoint string, body, v any) error {
	return c.sendEnvelope(ctx, method, endpoint, c.authHd, body, v)
}

// OAuthError carries the OAuth business code from a non-zero envelope.
type OAuthError struct {
	Code    int
	Message string
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("oauth code=%d msg=%q", e.Code, e.Message)
}

// MoemoepointResult mirrors POST /users/:id/moemoepoint's data field
// (06-moemoepoint.md §3.1). Applied=false means the idempotency key matched
// an existing entry and nothing was re-applied.
type MoemoepointResult struct {
	UserID  int  `json:"user_id"`
	Balance int  `json:"balance"`
	Applied bool `json:"applied"`
}

// AdjustMoemoepoint adjusts a user's unified moemoepoint balance on the OAuth
// single source (06-moemoepoint.md §3.1). `delta` is signed, non-zero,
// |delta| ≤ 1,000,000. `reason` must be an s2s reason (daily_checkin / liked /
// content_approved / content_removed — admin_*/migration are OAuth-reserved).
// `idempotencyKey` is a caller-generated stable key per business event.
// `source_app` is derived server-side from the authenticated client.
func (c *Client) AdjustMoemoepoint(
	ctx context.Context,
	userID, delta int,
	reason, ref, idempotencyKey string,
) (MoemoepointResult, error) {
	var out MoemoepointResult
	endpoint := fmt.Sprintf("%s/users/%d/moemoepoint", c.cfg.BaseURL, userID)
	err := c.doJSON(ctx, "POST", endpoint, map[string]any{
		"delta":           delta,
		"reason":          reason,
		"ref":             ref,
		"idempotency_key": idempotencyKey,
	}, &out)
	return out, err
}

// GetMoemoepoint reads a user's unified balance from OAuth (06 §3.2).
func (c *Client) GetMoemoepoint(ctx context.Context, userID int) (int, error) {
	var out struct {
		Balance int `json:"balance"`
	}
	endpoint := fmt.Sprintf("%s/users/%d/moemoepoint", c.cfg.BaseURL, userID)
	if err := c.do(ctx, "GET", endpoint, &out); err != nil {
		return 0, err
	}
	return out.Balance, nil
}

// MoemoepointLogEntry is the s2s slim view of one ledger row from
// GET /users/:id/moemoepoint/log (06-moemoepoint.md §3.2). `note` and
// `actor_user_id` are intentionally absent from the s2s view (they may carry
// admin penalty notes), so they're omitted here too.
type MoemoepointLogEntry struct {
	ID        int64  `json:"id"`
	Delta     int    `json:"delta"`
	Reason    string `json:"reason"`
	SourceApp string `json:"source_app"`
	Ref       string `json:"ref"`
	CreatedAt string `json:"created_at"`
	// IsLocal marks entries this app itself granted (source_app == our
	// client_id). Only these have a `ref` that resolves to a page on THIS site,
	// so the FE links the ref id only when IsLocal. Not part of OAuth's payload
	// — filled in by MoemoepointLog below.
	IsLocal bool `json:"is_local"`
}

// MoemoepointLogPage is one page of the ledger as OAuth returns it
// (data = {items, has_more}). HasMore drives cursor pagination without an
// extra empty fetch.
type MoemoepointLogPage struct {
	Items   []MoemoepointLogEntry `json:"items"`
	HasMore bool                  `json:"has_more"`
}

// MoemoepointLog pulls one page of a user's unified moemoepoint ledger from
// OAuth (06 §3.2). Cursor pagination: beforeID=0 fetches the newest page, then
// pass the last returned entry's ID for older pages. reason="" = no filter.
func (c *Client) MoemoepointLog(
	ctx context.Context,
	userID, limit, beforeID int,
	reason string,
) (MoemoepointLogPage, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	if beforeID > 0 {
		q.Set("before_id", strconv.Itoa(beforeID))
	}
	if reason != "" {
		q.Set("reason", reason)
	}
	endpoint := fmt.Sprintf("%s/users/%d/moemoepoint/log?%s", c.cfg.BaseURL, userID, q.Encode())

	var page MoemoepointLogPage
	if err := c.do(ctx, "GET", endpoint, &page); err != nil {
		return MoemoepointLogPage{}, err
	}
	if page.Items == nil {
		page.Items = []MoemoepointLogEntry{}
	}
	// Flag entries this app granted — their ref resolves to a local page.
	for i := range page.Items {
		page.Items[i].IsLocal = page.Items[i].SourceApp == c.cfg.ClientID
	}
	return page, nil
}

// cacheStore writes the batch result into hot + miss caches.
func (c *Client) cacheStore(bd batchData, now time.Time) {
	hotExp := now.Add(c.hotTTL)
	missExp := now.Add(c.negTTL)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, u := range bd.Users {
		c.hot[u.ID] = cacheEntry{user: u, expire: hotExp}
		delete(c.miss, u.ID) // a previous miss was wrong, clear it
	}
	for _, id := range bd.NotFound {
		c.miss[id] = missExp
	}
}

// Invalidate drops a user id from hot+miss cache. Use after explicit
// updates (admin ban, etc.) so the next read goes back to OAuth.
func (c *Client) Invalidate(ids ...int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range ids {
		delete(c.hot, id)
		delete(c.miss, id)
	}
}

func joinInts(xs []int, sep string) string {
	var b strings.Builder
	for i, x := range xs {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(strconv.Itoa(x))
	}
	return b.String()
}

func joinIntsForKey(xs []int) string {
	// Sort would be ideal for cache key normalization; chunks are short so
	// just use the joined string. Singleflight benefit only kicks in when
	// two concurrent callers happen to pass the same shard.
	return joinInts(xs, ",")
}

// CreatorApplication mirrors OAuth's creator_applications row (the fields the
// forum/moyu surface to the user). Acted on behalf of the END USER, not via
// client credentials. Contract owned by OAuth (not yet mirrored under docs/).
type CreatorApplication struct {
	ID            int             `json:"id"`
	UserID        int             `json:"user_id"`
	Source        string          `json:"source"`
	Status        string          `json:"status"`
	Evidence      json.RawMessage `json:"evidence,omitempty"`
	Message       string          `json:"message"`
	DeclineReason string          `json:"decline_reason"`
	ReviewedAt    *string         `json:"reviewed_at,omitempty"`
	CreatedAt     string          `json:"created_at"`
}

// doJSONWithToken is do/doJSON but authenticates as the END USER (Bearer)
// rather than the client-credentials Basic header — for acting-on-behalf-of-
// user calls. A nil body sends no request body (GET).
func (c *Client) doJSONWithToken(ctx context.Context, method, endpoint, token string, body, v any) error {
	return c.sendEnvelope(ctx, method, endpoint, "Bearer "+token, body, v)
}

// CreateCreatorApplication files a creator-role application as the user (Bearer
// token). `evidence` is the downstream-computed proof of which criterion was met.
func (c *Client) CreateCreatorApplication(ctx context.Context, token, source string, evidence json.RawMessage, message string) (*CreatorApplication, error) {
	var out CreatorApplication
	endpoint := c.cfg.BaseURL + "/creator/applications"
	body := map[string]any{"source": source, "evidence": evidence, "message": message}
	if err := c.doJSONWithToken(ctx, "POST", endpoint, token, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMyCreatorApplication returns the user's latest creator application, or nil
// if they've never applied.
func (c *Client) GetMyCreatorApplication(ctx context.Context, token string) (*CreatorApplication, error) {
	var out *CreatorApplication
	endpoint := c.cfg.BaseURL + "/creator/applications/me"
	if err := c.doJSONWithToken(ctx, "GET", endpoint, token, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
