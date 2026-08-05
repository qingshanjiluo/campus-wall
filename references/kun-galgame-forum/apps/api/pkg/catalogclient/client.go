// Package catalogclient is a thin S2S SDK for the infra Catalog service
// (kun-galgame-infra cmd/catalog). Its live surface is the editing engine's
// actor-assertion edit face (edit.go) — the one channel a first-party BFF
// speaks (doc 23 §5). Auth is HTTP Basic with an OAuth client_id/secret — the
// same first-party credential pair kungal already uses for the other infra S2S
// faces (image / artifact / trust). There is no generated client, so the calls
// are hand-written against the committed contract
// (kun-galgame-infra/docs/catalog + the catalog handler code), reusing the
// shared {code,message,data} house envelope.
//
// Payloads that need no reshape are forwarded VERBATIM as json.RawMessage: the
// catalog contract is the forum's exit contract, so no DTO is re-declared for
// them — nothing to rename, nothing to drift.
package catalogclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config bundles connection settings (created in app.go from config). The Basic
// credentials are kungal's OAuth client_id/secret.
type Config struct {
	BaseURL      string // catalog service base, e.g. http://127.0.0.1:9281 (no trailing slash)
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client // optional; defaults to a 10s-timeout client
}

// Client is a thin wrapper over the catalog HTTP read API.
type Client struct {
	basicAuth  string
	baseURL    string
	httpClient *http.Client
}

// New constructs a Client. Empty BaseURL/credentials = a no-op client whose
// calls return ErrNotConfigured, so kungal degrades gracefully when the catalog
// service isn't wired (dev, or before infra onboarding).
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	var ba string
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		ba = "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":"+cfg.ClientSecret))
	}
	return &Client{
		basicAuth:  ba,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: hc,
	}
}

// Configured reports whether the client can reach the catalog service S2S.
func (c *Client) Configured() bool { return c.baseURL != "" && c.basicAuth != "" }

// Sentinel errors callers can errors.Is against, so the BFF can map each to the
// right browser-facing status (real 404 vs. degrade-to-503 for a down catalog).
var (
	ErrNotConfigured = errors.New("catalogclient: not configured (empty base URL or credentials)")
	ErrUnauthorized  = errors.New("catalogclient: unauthorized (check client_id/secret)")
	ErrNotFound      = errors.New("catalogclient: entity not found")
	ErrUpstream      = errors.New("catalogclient: catalog service error")
)

// getData performs a Basic-authed GET and returns the envelope's `data` field
// verbatim so the shape stays byte-for-byte the catalog contract. It never
// 500s the calling page on its own: an unreachable / erroring catalog maps to a
// sentinel error the BFF degrades on (503), and a missing entity to ErrNotFound.
func (c *Client) getData(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.basicAuth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Transport-level failure: catalog unreachable / DNS / timeout.
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to envelope parse
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, fmt.Errorf("%w (status %d)", ErrUpstream, resp.StatusCode)
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("%w: malformed envelope", ErrUpstream)
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("%w (code %d): %s", ErrUpstream, env.Code, env.Message)
	}
	return env.Data, nil
}
