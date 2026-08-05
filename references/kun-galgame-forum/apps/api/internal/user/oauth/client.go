package oauth

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"kun-galgame-api/pkg/config"
)

// oauthHTTPTimeout caps every OAuth-server roundtrip (token exchange /
// refresh / revoke / userinfo) at 10s. Without this every login + every
// authenticated request would block indefinitely if the OAuth server hung,
// since the four hot paths all run synchronously in the request hot path.
const oauthHTTPTimeout = 10 * time.Second

// Failure classes kungal branches on (banned vs. refresh-token-expired vs.
// invalid-grant vs. everything-else-treated-as-transient). These are kungal's
// OWN discriminants: the protocol endpoints send RFC error strings, which
// oauthErrToCode maps onto these numbers; the house endpoints still send them
// literally. The full house code list is in docs/oauth/api-reference.md.
const (
	CodeAccountBanned       = 10014 // HTTP 403
	CodeRefreshTokenExpired = 10003 // HTTP 401 — needs user to fully re-login
	CodeInvalidToken        = 10002 // HTTP 401 — bad token / client_id mismatch
	CodeInvalidGrant        = 15005 // HTTP 400 — client missing `refresh_token` grant
	CodeInvalidClientSecret = 15008 // HTTP 400 — confidential client misconfigured
)

// Error is a structured OAuth-server error. It captures the failure class
// (when the response body was parseable) so middleware can branch on
// "banned" vs "transient" vs "client misconfig". Non-OAuth failures
// (network, body unreadable) are also wrapped in Error with Code == 0;
// IsTransient treats those as retryable.
type Error struct {
	Code       int    // failure class (see the constants); 0 when unparseable
	HTTPStatus int    // 0 for network errors
	Message    string // OAuth-supplied message (best effort)
}

func (e *Error) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("oauth: code=%d http=%d msg=%q", e.Code, e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("oauth: http=%d msg=%q", e.HTTPStatus, e.Message)
}

// IsBanned reports whether err is an "account banned" response.
// Callers should surface this distinctly (e.g. show a banned page rather
// than redirecting to /login, since logging in again hits the same error).
//
// Two shapes mean the same thing: the house 10014 (from /auth/me*), and a bare
// HTTP 403 from /oauth/userinfo — RFC 6750 has no error code carrying "banned",
// so the status is the signal there. The OAuth server uses 403 on these routes
// only for a banned account, so keying on it keeps the distinct banned page
// rather than degrading it to a re-login loop. (moyu classifies it the same way.)
func IsBanned(err error) bool {
	var oe *Error
	if !stderrors.As(err, &oe) {
		return false
	}
	return oe.Code == CodeAccountBanned || oe.HTTPStatus == http.StatusForbidden
}

// IsRefreshTokenDead reports whether err means the refresh token is
// permanently unusable — the user must log in again from scratch. Covers
// "token expired", "invalid token" (e.g. client_id mismatch), and
// "invalid grant" (e.g. refresh_token grant not allowed for this client).
func IsRefreshTokenDead(err error) bool {
	var oe *Error
	if !stderrors.As(err, &oe) {
		return false
	}
	switch oe.Code {
	case CodeRefreshTokenExpired, CodeInvalidToken, CodeInvalidGrant, CodeInvalidClientSecret:
		return true
	}
	return false
}

// IsTransient reports whether err looks recoverable on a retry (network
// glitch, OAuth restart, 5xx, unparseable body). The middleware uses this
// to decide whether to keep the local session alive across the failure.
func IsTransient(err error) bool {
	var oe *Error
	if !stderrors.As(err, &oe) {
		// Plain network errors (rare path; usually wrapped) — treat as transient.
		return true
	}
	if oe.HTTPStatus == 0 || oe.HTTPStatus >= 500 {
		return true
	}
	// Unparseable body on a 4xx → can't tell, lean transient.
	if oe.Code == 0 {
		return true
	}
	return false
}

// Client calls the OAuth server via HTTP.
// It is a thin transport layer: raw HTTP plus the two response readers
// (decodeProtocol / decodeHouse). No business logic lives here.
type Client struct {
	cfg        config.OAuthConfig
	httpClient *http.Client
}

// NewClient constructs an OAuth HTTP client with the given configuration.
// The HTTP client carries a per-request timeout so a hung OAuth server can't
// stall login / token refresh / logout indefinitely.
func NewClient(cfg config.OAuthConfig) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: oauthHTTPTimeout},
	}
}

// The OAuth server speaks TWO different wire formats, split by endpoint, and
// this client calls both — so it has one reader per face and they must not be
// mixed up:
//
//   - decodeProtocol → /oauth/*  : RFC 6749 / RFC 6750. Bare top-level JSON on
//     success, {error, error_description} on failure.
//   - decodeHouse    → /auth/me* : the house {code,message,data} envelope, which
//     is this platform's private API convention and is NOT going away.
//
// Both return a typed *Error so callers keep branching through
// IsBanned / IsRefreshTokenDead / IsTransient regardless of which face answered.

// decodeProtocol reads a reply from an OAuth/OIDC protocol endpoint.
func decodeProtocol(resp *http.Response) (json.RawMessage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{HTTPStatus: resp.StatusCode, Message: "读取响应体失败: " + err.Error()}
	}

	if resp.StatusCode == http.StatusOK {
		// The whole body IS the payload.
		return json.RawMessage(body), nil
	}

	var p struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if jerr := json.Unmarshal(body, &p); jerr != nil {
		// Unparseable body — status known, error unknown. Treated as transient.
		return nil, &Error{
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("解析响应失败: %v, body=%s", jerr, truncateBody(body)),
		}
	}
	msg := p.ErrorDescription
	if msg == "" {
		msg = p.Error
	}
	return nil, &Error{Code: oauthErrToCode(p.Error), HTTPStatus: resp.StatusCode, Message: msg}
}

// decodeHouse reads a reply from a house endpoint (/auth/me, /auth/me/avatar).
// Success means HTTP 200 AND code == 0 AND a non-empty data payload.
func decodeHouse(resp *http.Response) (json.RawMessage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{HTTPStatus: resp.StatusCode, Message: "读取响应体失败: " + err.Error()}
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if jerr := json.Unmarshal(body, &env); jerr != nil {
		return nil, &Error{
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("解析响应失败: %v, body=%s", jerr, truncateBody(body)),
		}
	}
	if resp.StatusCode == http.StatusOK && env.Code == 0 && len(env.Data) > 0 {
		return env.Data, nil
	}
	return nil, &Error{Code: env.Code, HTTPStatus: resp.StatusCode, Message: env.Message}
}

// oauthErrToCode maps a standard RFC 6749 §5.2 / RFC 6750 §3.1 error string to
// the failure class kungal branches on. invalid_token (a rejected bearer token
// on /oauth/userinfo), invalid_grant / unauthorized_client (the OAuth server's
// grant-allowlist rejection) and invalid_client all mean the credential is
// unusable (→ re-login); anything else maps to 0 (unknown → transient).
//
// Ban handling: RFC 6750 has no error code for "banned", so a banned user
// arrives as invalid_token — IsBanned recovers the distinction from the HTTP
// 403 instead. On the token endpoint a ban surfaces as invalid_grant with no
// 403, so there it degrades to the generic re-login path. The ban is enforced
// either way; only the wording the user sees differs.
func oauthErrToCode(errStr string) int {
	switch errStr {
	case "invalid_token":
		// Without this a 401 from /oauth/userinfo lands on code 0, which
		// IsTransient reads as retryable — kungal would keep a session with a
		// permanently dead token alive and retry it forever.
		return CodeInvalidToken
	case "invalid_grant", "unauthorized_client":
		return CodeInvalidGrant
	case "invalid_client":
		return CodeInvalidClientSecret
	default:
		return 0
	}
}

// truncateBody trims a response body to a sane length for error messages
// so logs don't blow up if OAuth returns a giant HTML error page.
func truncateBody(b []byte) string {
	const max = 256
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "...(truncated)"
}

// TokenResponse represents the token data inside the OAuth response wrapper.
// /oauth/token returns { code: 0, message: "成功", data: { access_token, ... } }
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// UserInfo represents the OAuth userinfo payload.
//
// IMPORTANT: kungal post-migration relies on the integer `id` (= OAuth
// users.id) and the `roles` array. The OIDC userinfo standard only
// requires `sub` (UUID). The OAuth team must extend /oauth/userinfo to
// include `id` and `roles` so kungal can derive its userID + admin role
// without a second round-trip.
type UserInfo struct {
	ID      int      `json:"id"`
	Sub     string   `json:"sub"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Picture string   `json:"picture"`
	Roles   []string `json:"roles"`
	// SiteRoles is the OAuth `site_roles` claim — site-scoped role names for THIS
	// client's site (contract docs/oauth/12-site-roles.md). Same shape as Roles;
	// omitted when the user has no grant on this site. It is merged into the
	// effective role set (pkg/role.Union) and can never contain admin/ren
	// (contract §3/§5.3), so the union only ever adds moderator/creator/custom.
	SiteRoles []string `json:"site_roles"`
	UpdatedAt int64    `json:"updated_at"`
}

// ExchangeCode exchanges an authorization code for access/refresh tokens.
// Returns a typed *Error on OAuth-side failures (see Error / IsBanned /
// IsTransient).
func (c *Client) ExchangeCode(code, codeVerifier string) (*TokenResponse, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  c.cfg.RedirectURI,
		"client_id":     c.cfg.ClientID,
		"client_secret": c.cfg.ClientSecret,
		"code_verifier": codeVerifier,
	}
	data, err := c.postProtocol("/oauth/token", payload)
	if err != nil {
		return nil, err
	}
	var tok TokenResponse
	if jerr := json.Unmarshal(data, &tok); jerr != nil {
		return nil, &Error{Message: "解析 token 响应失败: " + jerr.Error()}
	}
	if tok.AccessToken == "" {
		return nil, &Error{Message: "token 响应缺 access_token"}
	}
	return &tok, nil
}

// FetchUserInfo retrieves the OAuth user info using an access token.
// Returns a typed *Error on OAuth-side failures.
func (c *Client) FetchUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", c.cfg.ServerURL+"/oauth/userinfo", nil)
	if err != nil {
		return nil, &Error{Message: "创建 userinfo 请求失败: " + err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &Error{Message: "请求 userinfo 失败: " + err.Error()}
	}
	defer resp.Body.Close()

	data, derr := decodeProtocol(resp)
	if derr != nil {
		return nil, derr
	}
	var info UserInfo
	if jerr := json.Unmarshal(data, &info); jerr != nil {
		return nil, &Error{Message: "解析 userinfo 响应失败: " + jerr.Error()}
	}
	return &info, nil
}

// RevokeToken revokes a refresh token against the OAuth server.
func (c *Client) RevokeToken(refreshToken string) error {
	payload, err := json.Marshal(map[string]string{"token": refreshToken})
	if err != nil {
		return fmt.Errorf("序列化 revoke 请求失败: %w", err)
	}
	req, err := http.NewRequest("POST", c.cfg.ServerURL+"/oauth/revoke", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建 revoke 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// RefreshOAuthToken refreshes the OAuth tokens using the refresh token.
// Returns a typed *Error on OAuth-side failures — middleware switches on
// IsBanned / IsRefreshTokenDead / IsTransient to decide whether to
// preserve or invalidate the local session.
func (c *Client) RefreshOAuthToken(refreshToken string) (*TokenResponse, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     c.cfg.ClientID,
		"client_secret": c.cfg.ClientSecret,
	}
	data, err := c.postProtocol("/oauth/token", payload)
	if err != nil {
		return nil, err
	}
	var tok TokenResponse
	if jerr := json.Unmarshal(data, &tok); jerr != nil {
		return nil, &Error{Message: "解析刷新响应失败: " + jerr.Error()}
	}
	if tok.AccessToken == "" {
		return nil, &Error{Message: "刷新响应缺 access_token"}
	}
	return &tok, nil
}

// PatchAuthMe calls PATCH /auth/me to update the authenticated user's
// profile. The body is any JSON-serialisable struct/map carrying the
// fields the user wants to change — OAuth treats omitted fields as
// "leave unchanged" so kungal can forward partial updates. Returns the
// raw refreshed user payload so callers can pass it back to the
// browser verbatim.
//
// docs/oauth/02-user-profile.md §PATCH /auth/me.
func (c *Client) PatchAuthMe(accessToken string, body any) (json.RawMessage, error) {
	payload, jerr := json.Marshal(body)
	if jerr != nil {
		return nil, &Error{Message: "序列化 PATCH /auth/me 请求失败: " + jerr.Error()}
	}
	req, rerr := http.NewRequest("PATCH", c.cfg.ServerURL+"/auth/me", bytes.NewReader(payload))
	if rerr != nil {
		return nil, &Error{Message: "创建 PATCH /auth/me 请求失败: " + rerr.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, derr := c.httpClient.Do(req)
	if derr != nil {
		return nil, &Error{Message: "请求 PATCH /auth/me 失败: " + derr.Error()}
	}
	defer resp.Body.Close()
	return decodeHouse(resp)
}

// UploadAvatar calls POST /auth/me/avatar with a pre-built multipart
// body. OAuth pipes the bytes to image_service, writes the resulting
// hash to the user row, and returns the image_service upload result
// (hash + variant URLs). kungal forwards the body unchanged.
//
// contentType is the value of the incoming request's Content-Type
// header (must carry the multipart boundary).
//
// docs/oauth/02-user-profile.md §POST /auth/me/avatar.
func (c *Client) UploadAvatar(accessToken string, body []byte, contentType string) (json.RawMessage, error) {
	req, rerr := http.NewRequest("POST", c.cfg.ServerURL+"/auth/me/avatar", bytes.NewReader(body))
	if rerr != nil {
		return nil, &Error{Message: "创建 POST /auth/me/avatar 请求失败: " + rerr.Error()}
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, derr := c.httpClient.Do(req)
	if derr != nil {
		return nil, &Error{Message: "请求 POST /auth/me/avatar 失败: " + derr.Error()}
	}
	defer resp.Body.Close()
	return decodeHouse(resp)
}

// postProtocol POSTs a JSON-serialized payload to OAuth and decodes the
// standard envelope. Used by ExchangeCode and RefreshOAuthToken — both
// hit /oauth/token with the same wire shape but different grant_type.
func (c *Client) postProtocol(path string, payload any) (json.RawMessage, error) {
	body, jerr := json.Marshal(payload)
	if jerr != nil {
		return nil, &Error{Message: "序列化请求失败: " + jerr.Error()}
	}
	req, rerr := http.NewRequest("POST", c.cfg.ServerURL+path, bytes.NewReader(body))
	if rerr != nil {
		return nil, &Error{Message: "创建请求失败: " + rerr.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, derr := c.httpClient.Do(req)
	if derr != nil {
		return nil, &Error{Message: "请求 OAuth 失败: " + derr.Error()}
	}
	defer resp.Body.Close()
	return decodeProtocol(resp)
}
