// Package communityclient is a thin S2S client for the infra community primitive
// (kun-galgame-infra cmd/community, database kun_community, prefix
// /api/v1/community). kungal is a site BFF: it authenticates with HTTP Basic
// client credentials (its OAuth client_id/secret), the tenant `site` is derived
// server-side from the client's oauth_clients.catalog_site binding (never on the
// wire), and the acting user rides in each request body (the BFF has already
// authenticated the user against the kungal session).
//
// Modeled on pkg/trustclient (the other infra S2S client here) and mirrors the
// letmoe consumer's communityclient shape; the {code,message,data} house
// envelope is shared with every other infra service. Only the ops the galgame
// comment migration needs are wrapped (resolve/list/reply/edit/delete/react/
// flag/boost) — the full contract is in
// kun-galgame-infra/docs/community + internal/platform/community/handler/s2s.go.
package communityclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config bundles connection settings (built in app.go). The Basic credentials
// are kungal's OAuth client_id/secret.
type Config struct {
	BaseURL      string // community S2S base, e.g. http://127.0.0.1:9282/api/v1/community (no trailing slash needed)
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // optional; defaults to an 8s-timeout client
}

// Sentinels callers branch on. ErrNotConfigured degrades a read to an empty
// list / a write to 503 (never a local-table fallback — the community primitive
// is the source of truth). ErrRateLimited is the TL0 newcomer sandbox cap (HTTP
// 429). ErrForbidden is a missing/mismatched site binding (HTTP 403).
var (
	ErrNotConfigured = errors.New("communityclient: not configured (empty base URL or credentials)")
	ErrRateLimited   = errors.New("communityclient: rate limited (TL0 sandbox cap)")
	ErrForbidden     = errors.New("communityclient: forbidden (client not bound to a site)")
)

// APIError carries the community house-envelope error (non-zero code) plus the
// HTTP status, for the rare caller that needs the specifics.
type APIError struct {
	Status int
	Code   int
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("communityclient: status=%d code=%d msg=%s", e.Status, e.Code, e.Msg)
}

// Client talks to the community S2S face with Basic client credentials.
type Client struct {
	http    *http.Client
	baseURL string
	authHdr string // "Basic <b64(id:secret)>", empty when creds are unset
}

// New builds the client. An empty base URL or empty creds → Configured() is
// false and every call returns ErrNotConfigured (the face degrades cleanly on a
// dev box / before infra onboarding).
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 8 * time.Second}
	}
	c := &Client{
		http:    hc,
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
	}
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		c.authHdr = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	}
	return c
}

// Configured reports whether the client has a base URL and credentials.
func (c *Client) Configured() bool { return c.baseURL != "" && c.authHdr != "" }

// envelope is the community house response { code, message, data }.
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do performs an authenticated JSON request and unmarshals the envelope's data
// into out (out may be nil for fire-and-forget). It maps 403→ErrForbidden and
// 429→ErrRateLimited, and any other non-2xx / non-zero code to *APIError.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("community request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("community decode: %w", err)
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return &APIError{Status: resp.StatusCode, Code: env.Code, Msg: env.Message}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("community decode data: %w", err)
		}
	}
	return nil
}

// query builds a "?a=b" suffix from non-empty pairs (empty values are omitted so
// the community-side defaults apply).
func query(pairs map[string]string) string {
	q := url.Values{}
	for k, v := range pairs {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// joinInt64 renders an id slice as a comma-separated query value ("1,2,3").
func joinInt64(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}
