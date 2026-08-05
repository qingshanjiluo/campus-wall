package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"kun-galgame-api/internal/user/oauth"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/role"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

type contextKey string

const (
	UserInfoKey         contextKey = "userInfo"
	OAuthAccessTokenKey contextKey = "oauthAccessToken"
)

// Session namespace constants. These MUST be distinct from moyu's
// (kun-galgame-patch) values: in local dev both sites run on
// 127.0.0.1 (cookies are domain-scoped, NOT port-scoped) and share one
// Redis. A shared cookie name + key prefix made kungal read/refresh/delete
// moyu's sessions (and vice versa) → cross-site logout with
// client_id_mismatch on the OAuth server. Keep these site-unique.
const (
	// SessionCookieName is the browser cookie holding the session id.
	// kungal: "kungal_session"; moyu: "moyu_session".
	SessionCookieName = "kungal_session"
	// SessionPrefix namespaces session keys in Redis so a shared Redis
	// instance can't collide kungal vs moyu. kungal: "kungal:session:".
	//
	// The ":v2:" generation suffix was added when UserInfo dropped the legacy
	// numeric `role` field for the OAuth named-role set (`Roles []string`).
	// Bumping the namespace abandons every pre-cutover session whose blob still
	// carries the old shape, forcing a clean re-login into the new shape (an
	// explicitly authorized one-time logout) — no decode shim required.
	SessionPrefix = "kungal:session:v2:"
	// SessionTTL is the SLIDING lifetime of a kungal session — applied to
	// both the Redis entry and the browser cookie, and re-extended while the
	// user is active (see renewSlidingSession). 90 days matches the OAuth
	// refresh_token default (oauth_clients.refresh_token_ttl_seconds=7776000),
	// so for an idle user the local session and the upstream grant lapse
	// together; for an active user both slide forward indefinitely. The hard
	// ceiling is the upstream refresh_token itself: once it can no longer
	// refresh, refreshSession deletes the session regardless of this window.
	//
	// Was a FIXED 7 days set once at login and never renewed — which logged
	// every user out exactly 7 days after login regardless of activity.
	SessionTTL = 90 * 24 * time.Hour
	// sessionRenewPrefix keys the per-session cookie-renewal throttle marker.
	// Kept OFF SessionPrefix so it never matches a "kungal:session:*" scan.
	sessionRenewPrefix = "kungal:session-renew:"
)

// SecureCookies controls whether the renewed session cookie is marked Secure
// (HTTPS-only). Set at startup from the server mode (see App.setupRoutes) —
// must be false in dev over plain HTTP or the browser silently drops the
// cookie. Mirrors the login handler's own secure flag (OAuthHandler.secure).
var SecureCookies = true

// SessionKey returns the Redis key for a session token.
func SessionKey(token string) string { return SessionPrefix + token }

// UserInfo represents the authenticated user extracted from session.
type UserInfo struct {
	ID    int    `json:"id"`
	Sub   string `json:"sub"` // OAuth UUID
	Name  string `json:"name"`
	Email string `json:"email"`
	// Roles is the EFFECTIVE role set: the OAuth `roles` claim UNIONed with this
	// site's `site_roles` claim (contracts 11-roles.md + 12-site-roles.md),
	// merged at session ingestion via pkg/role.Union. An UNORDERED SET of name
	// strings; the implicit `user` default never appears, so a plain logged-in
	// user has an empty slice. Authorization is a capability over this set via
	// pkg/role (CanModerate / CanAdminister / IsCreator) — never a number/rank.
	// site_roles can never carry admin/ren, so the merge only adds mod/creator.
	Roles []string `json:"roles"`
}

// SessionData is stored in Redis under SessionKey(token).
type SessionData struct {
	UserInfo
	OAuthAccessToken  string `json:"oauth_access_token"`
	OAuthRefreshToken string `json:"oauth_refresh_token"`
	OAuthExpiresAt    int64  `json:"oauth_expires_at"`
}

// Auth creates a middleware that validates the session cookie.
// It looks up the session in Redis and attaches UserInfo to the context.
//
// Take an *oauth.Client (the same one AuthService uses) so that token
// refresh logic lives in exactly one place — see oauth.Client.RefreshOAuthToken.
// Identity (name / avatar / etc.) is OAuth-owned post-migration; mappers
// fetch via pkg/userclient.
func Auth(rdb *redis.Client, oauthClient *oauth.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Cookies(SessionCookieName)
		if token == "" {
			return response.Error(c, errors.ErrAuthExpired())
		}

		ctx := c.Context()
		val, err := rdb.Get(ctx, SessionKey(token)).Result()
		if err != nil {
			return response.Error(c, errors.ErrAuthExpired())
		}

		var session SessionData
		if err := json.Unmarshal([]byte(val), &session); err != nil {
			return response.Error(c, errors.ErrAuthExpired())
		}

		// Refresh the OAuth access token if it's expired (or within a 30s
		// grace window of expiry — see refreshSkew below). This is the hot
		// path that runs on every authenticated request, so the logic
		// needs to:
		//   (a) handle concurrent expiry across multiple in-flight requests
		//       without doing N parallel refresh round-trips, and
		//   (b) survive transient OAuth failures without killing the
		//       session and kicking the user out for an OAuth hiccup.
		//
		// Strategy: SETNX-based single-flight lock. The winner does the
		// refresh; losers wait (poll) until the winner publishes the new
		// session, then proceed with the fresh tokens. On refresh failure
		// we return 205 to THIS request but leave the session intact so
		// the next request can retry — only a permanently-invalid refresh
		// token will keep failing, and we'd rather get many 205s during a
		// transient outage than auto-logout every active user.
		const refreshSkew = 30 * time.Second
		needsRefresh := session.OAuthExpiresAt > 0 &&
			time.Now().Add(refreshSkew).Unix() > session.OAuthExpiresAt
		if needsRefresh {
			lockKey := "refresh_lock:" + token
			// Lock TTL must exceed the OAuth client's HTTP timeout (10s) so
			// the lock isn't auto-released mid-refresh — otherwise a second
			// request would grab it and call OAuth with a refresh token
			// that's already been rotated by the first.
			locked, _ := rdb.SetNX(ctx, lockKey, "1", 15*time.Second).Result()
			if locked {
				if err := refreshSession(ctx, rdb, oauthClient, token, &session); err != nil {
					rdb.Del(ctx, lockKey)
					return response.Error(c, err)
				}
				rdb.Del(ctx, lockKey)
			} else {
				if err := waitForRefresh(ctx, rdb, lockKey, token, &session); err != nil {
					return response.Error(c, err)
				}
			}
		}

		// Rolling session: while the user is active, slide the cookie + Redis
		// TTL forward so a returning user is never bounced as long as they
		// came back within SessionTTL. Throttled to once per half-window.
		renewSlidingSession(c, rdb, token)

		c.Locals(string(UserInfoKey), &session.UserInfo)
		// Expose the session's OAuth access token to handlers that need to
		// forward authority to the galgame service. Sourcing this from Redis
		// (rather than a client-supplied X-OAuth-Token header) guarantees
		// the token's subject matches the kun_session cookie holder.
		c.Locals(string(OAuthAccessTokenKey), session.OAuthAccessToken)
		return c.Next()
	}
}

// OptionalAuth is like Auth but does not fail if no session is present.
// If a valid session exists, UserInfo is attached; otherwise the request proceeds.
func OptionalAuth(rdb *redis.Client, oauthClient *oauth.Client) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Cookies(SessionCookieName)
		if token == "" {
			return c.Next()
		}

		ctx := c.Context()
		val, err := rdb.Get(ctx, SessionKey(token)).Result()
		if err != nil {
			return c.Next()
		}

		var session SessionData
		if err := json.Unmarshal([]byte(val), &session); err != nil {
			return c.Next()
		}

		// Best-effort refresh. optAuth read paths forward the session's OAuth
		// access token to the galgame service; without a refresh an EXPIRED token
		// would be forwarded and rejected as anonymous (a logged-in user could
		// not read their own drafts until some mandatory-Auth request happened
		// to refresh). Unlike Auth(), failure here is NON-fatal: if the token
		// can't be refreshed we serve the read as ANONYMOUS rather than 205 —
		// public reads are unaffected, and we never forward a dead token.
		const refreshSkew = 30 * time.Second
		if session.OAuthExpiresAt > 0 &&
			time.Now().Add(refreshSkew).Unix() > session.OAuthExpiresAt {
			var refreshErr *errors.AppError
			lockKey := "refresh_lock:" + token
			if locked, _ := rdb.SetNX(ctx, lockKey, "1", 15*time.Second).Result(); locked {
				refreshErr = refreshSession(ctx, rdb, oauthClient, token, &session)
				rdb.Del(ctx, lockKey)
			} else {
				refreshErr = waitForRefresh(ctx, rdb, lockKey, token, &session)
			}
			if refreshErr != nil {
				return c.Next() // serve anonymous; don't forward a stale token
			}
		}

		// Rolling session: same sliding renewal as Auth() for logged-in users
		// browsing optAuth routes (anonymous callers already returned above).
		renewSlidingSession(c, rdb, token)

		c.Locals(string(UserInfoKey), &session.UserInfo)
		// Mirror Auth(): also attach the session's OAuth access token so
		// GetAccessToken works on optAuth routes for logged-in users.
		// Without this, optAuth handlers that forward authority to galgame
		// (e.g. GET /galgame/:gid → GetDetail) always sent an empty
		// token, so galgame saw an anonymous caller and a submitter could
		// not open their own status=3/4 draft (20001 "Galgame 不存在").
		// Anonymous callers still hit the early c.Next() above, so this
		// is purely additive — public reads are unchanged.
		c.Locals(string(OAuthAccessTokenKey), session.OAuthAccessToken)
		return c.Next()
	}
}

// GetUser extracts UserInfo from the Fiber context. Returns nil if not authenticated.
func GetUser(c fiber.Ctx) *UserInfo {
	info, ok := c.Locals(string(UserInfoKey)).(*UserInfo)
	if !ok {
		return nil
	}
	return info
}

// MustGetUser extracts UserInfo or returns an auth error.
func MustGetUser(c fiber.Ctx) (*UserInfo, *errors.AppError) {
	info := GetUser(c)
	if info == nil {
		return nil, errors.ErrAuthExpired()
	}
	return info, nil
}

// GetAccessToken returns the session-stored OAuth access token attached by
// Auth middleware. Returns empty when no valid session is present (e.g.
// OptionalAuth path with anonymous user). Callers that forward authority to
// the galgame service MUST source the token from here — never from a
// client-supplied header — so the token subject is guaranteed to match the
// kun_session cookie holder.
func GetAccessToken(c fiber.Ctx) string {
	tok, _ := c.Locals(string(OAuthAccessTokenKey)).(string)
	return tok
}

// refreshSession is the lock-winner path: actually call OAuth, mutate the
// passed-in session in place, and write the result back to Redis.
//
// Failure branches (matters for the user-experience side of the 401 loop):
//   - oauth.IsBanned(err)            → delete session, surface CodeBanned;
//     frontend stops the user from looping
//     through /login (a re-login hits 10014
//     again at the very next refresh).
//   - oauth.IsRefreshTokenDead(err)  → delete session, surface 205; user
//     must do a fresh /oauth/authorize.
//     Covers refresh_token expired, client_id
//     mismatch, invalid_grant, secret mismatch.
//   - oauth.IsTransient(err)         → keep session, surface 205; the next
//     request retries the refresh. This is
//     what makes OAuth restarts / network
//     hiccups not auto-logout every user.
func refreshSession(
	ctx context.Context,
	rdb *redis.Client,
	oauthClient *oauth.Client,
	token string,
	session *SessionData,
) *errors.AppError {
	refreshed, err := oauthClient.RefreshOAuthToken(session.OAuthRefreshToken)
	if err != nil {
		switch {
		case oauth.IsBanned(err):
			slog.Warn("OAuth 刷新返回账号封禁", "error", err)
			rdb.Del(ctx, SessionKey(token))
			return errors.ErrAccountBanned()
		case oauth.IsRefreshTokenDead(err):
			slog.Warn("OAuth refresh_token 不可恢复, 清除 session", "error", err)
			rdb.Del(ctx, SessionKey(token))
			return errors.ErrAuthExpired()
		default:
			// Transient: don't touch the session, let the next request retry.
			slog.Warn("OAuth token 刷新失败 (保留 session, 留给下次请求重试)",
				"error", err)
			return errors.ErrAuthExpired()
		}
	}
	session.OAuthAccessToken = refreshed.AccessToken
	session.OAuthRefreshToken = refreshed.RefreshToken
	session.OAuthExpiresAt = time.Now().Unix() + int64(refreshed.ExpiresIn)

	// Re-read roles from fresh userinfo so an upstream change — a demoted admin /
	// promoted mod, OR a site-role grant/revoke — takes effect at the next token
	// refresh instead of being frozen at login for the whole session. Roles ∪
	// site_roles = the effective set (contract 12-site-roles §4/§5.1). A ban
	// surfaced here is enforced immediately; a transient userinfo failure is
	// non-fatal — the tokens already refreshed, so we keep the cached roles.
	if info, uErr := oauthClient.FetchUserInfo(refreshed.AccessToken); uErr == nil {
		session.Roles = role.Union(info.Roles, info.SiteRoles)
	} else if oauth.IsBanned(uErr) {
		slog.Warn("刷新后 userinfo 返回账号封禁, 清除 session", "error", uErr)
		rdb.Del(ctx, SessionKey(token))
		return errors.ErrAccountBanned()
	} else {
		slog.Warn("刷新后拉取 userinfo 失败, 保留旧 roles", "error", uErr)
	}

	data, mErr := json.Marshal(session)
	if mErr != nil {
		slog.Error("序列化 session 失败", "error", mErr)
		return errors.ErrInternal("服务器内部错误")
	}
	// Bump the Redis TTL so it never expires before the cookie. The cookie's
	// own sliding renewal is handled separately by renewSlidingSession (which
	// also persists RenewedAt) — RenewedAt set on this struct is preserved
	// here because we marshal the whole session.
	rdb.Set(ctx, SessionKey(token), data, SessionTTL)
	return nil
}

// renewSlidingSession implements the rolling half of the session-timeout
// model: while the user is active it slides the cookie + Redis TTL forward, so
// a returning user is never logged out as long as they came back within
// SessionTTL of their last visit. The absolute ceiling is the upstream OAuth
// refresh_token — once that can no longer refresh, refreshSession deletes the
// session and the user re-logs in regardless of this window.
//
// To avoid a Set-Cookie on every request it renews at most once per
// half-window — the same heuristic ASP.NET Core's SlidingExpiration uses. The
// throttle is a marker key whose SetNX succeeds only after the previous marker
// has expired (i.e. >SessionTTL/2 since the last renewal).
//
// The TTL is slid with EXPIRE, NOT a blob rewrite, so this can never race a
// concurrent token refresh / rotation and clobber freshly-rotated tokens.
// Best-effort: a Redis hiccup just skips this round; the cookie keeps its
// current expiry and the next qualifying request retries. Call only with a
// validated session present.
func renewSlidingSession(c fiber.Ctx, rdb *redis.Client, token string) {
	ctx := c.Context()
	if ok, _ := rdb.SetNX(ctx, sessionRenewPrefix+token, "1", SessionTTL/2).Result(); !ok {
		return
	}
	rdb.Expire(ctx, SessionKey(token), SessionTTL)
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		MaxAge:   int(SessionTTL.Seconds()),
		HTTPOnly: true,
		Secure:   SecureCookies,
		SameSite: "Lax",
		Path:     "/",
	})
}

// waitForRefresh is the lock-loser path. Another request is currently
// refreshing this user's token; instead of racing through with the stale
// access token (which would just generate downstream 401s from the galgame
// service), we poll until either:
//
//   - the session in Redis has a fresh OAuthExpiresAt → proceed with the
//     freshly-published tokens; or
//   - the lock key disappears with the session still expired → the winner
//     failed; surface as auth-expired so the next request can retry; or
//   - we exceed the wait deadline → also surface as auth-expired.
//
// The poll interval (150ms) gives sub-second responsiveness once the
// winner publishes. The deadline (12s) sits between the OAuth client's
// 10s HTTP timeout and the 15s SETNX TTL, so a still-pending refresh has
// time to finish but we give up before the lock would auto-expire (after
// which we wouldn't be able to distinguish "refresh failed" from
// "refresh still in flight").
func waitForRefresh(
	ctx context.Context,
	rdb *redis.Client,
	lockKey, token string,
	session *SessionData,
) *errors.AppError {
	deadline := time.Now().Add(12 * time.Second)
	for {
		time.Sleep(150 * time.Millisecond)

		val, err := rdb.Get(ctx, SessionKey(token)).Result()
		if err != nil {
			return errors.ErrAuthExpired()
		}
		if uErr := json.Unmarshal([]byte(val), session); uErr != nil {
			return errors.ErrAuthExpired()
		}

		// Refresh published — fall through to the request handler.
		if session.OAuthExpiresAt > time.Now().Unix() {
			return nil
		}

		// Lock released but session still expired → winner's refresh failed.
		// Fail fast (don't wait full deadline) so the user just retries.
		exists, _ := rdb.Exists(ctx, lockKey).Result()
		if exists == 0 {
			return errors.ErrAuthExpired()
		}

		if time.Now().After(deadline) {
			return errors.ErrAuthExpired()
		}
	}
}
