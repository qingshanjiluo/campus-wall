// Package linkcheck is a thin s2s client for the kungal-link-live-checker
// service — the gate behind "report resource expired". The checker returns a
// conservative alive/dead/unknown verdict for a netdisk share link; its iron
// law is that only `dead` is an explicit upstream "share is gone", and anything
// uncertain is `unknown` (never a false `dead`). kungal therefore trusts `dead`
// to auto-expire, `alive` to reject a false report, and treats `unknown` — and
// every transport/decode error here — as "fall back to the legacy flow".
package linkcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Status is the checker's conservative three-state verdict.
type Status string

const (
	StatusAlive   Status = "alive"
	StatusDead    Status = "dead"
	StatusUnknown Status = "unknown"
)

// Config is the s2s connection config (see config.LinkCheckerConfig).
type Config struct {
	BaseURL string
	APIKey  string
	// CFAccessClientID / Secret are the Cloudflare Access service-token headers.
	// The checker sits behind CF Access, so without these every request is
	// rejected at the edge (401) before the Bearer key is even seen. Optional —
	// leave empty for a checker not behind CF Access.
	CFAccessClientID     string
	CFAccessClientSecret string
	// Timeout bounds ONE batch call; the checker probes each link's netdisk
	// share API serially, so allow a few seconds per link. Zero → defaultTimeout.
	Timeout time.Duration
}

const defaultTimeout = 12 * time.Second

// Client calls the checker's POST /v1/check/batch endpoint with Bearer auth
// (plus Cloudflare Access service-token headers when configured).
type Client struct {
	baseURL  string
	apiKey   string
	cfID     string
	cfSecret string
	http     *http.Client
}

// New builds a Client. Construct it ONLY when BaseURL and APIKey are both set —
// an unconfigured gate must be skipped by the caller, not called.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		cfID:     cfg.CFAccessClientID,
		cfSecret: cfg.CFAccessClientSecret,
		http:     &http.Client{Timeout: timeout},
	}
}

type checkItem struct {
	URL      string `json:"url"`
	Passcode string `json:"passcode,omitempty"`
}

type batchRequest struct {
	Items []checkItem `json:"items"`
}

// Result mirrors one of the checker's per-link verdicts.
type Result struct {
	Provider string `json:"provider"`
	Status   Status `json:"status"`
	Reason   string `json:"reason"`
}

type batchResponse struct {
	Results []Result `json:"results"`
}

// CheckShare returns ONE aggregated verdict for a resource's links (which share
// the same passcode). The aggregation is conservative by design:
//
//   - any link alive → Alive   (resource still reachable → reject the report)
//   - all links dead → Dead    (every mirror verified gone → safe to expire)
//   - anything else  → Unknown (mixed / uncertain / no links / transport error
//     → the caller falls back to its legacy mechanism)
func (c *Client) CheckShare(ctx context.Context, urls []string, passcode string) Status {
	if len(urls) == 0 {
		return StatusUnknown
	}
	items := make([]checkItem, len(urls))
	for i, u := range urls {
		items[i] = checkItem{URL: u, Passcode: passcode}
	}
	body, err := json.Marshal(batchRequest{Items: items})
	if err != nil {
		return StatusUnknown
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/check/batch", bytes.NewReader(body))
	if err != nil {
		return StatusUnknown
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	// Cloudflare Access service token (edge auth) — must precede the Bearer key,
	// which CF Access never even sees until these pass.
	if c.cfID != "" && c.cfSecret != "" {
		req.Header.Set("CF-Access-Client-Id", c.cfID)
		req.Header.Set("CF-Access-Client-Secret", c.cfSecret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return StatusUnknown // checker down / timeout → fall back
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return StatusUnknown
	}
	var out batchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return StatusUnknown
	}
	return aggregate(out.Results)
}

// aggregate folds per-link verdicts into the resource-level verdict. See
// CheckShare for the rules. Split out so the conservative logic is unit-tested.
func aggregate(results []Result) Status {
	if len(results) == 0 {
		return StatusUnknown
	}
	allDead := true
	for _, r := range results {
		if r.Status == StatusAlive {
			return StatusAlive
		}
		if r.Status != StatusDead {
			allDead = false
		}
	}
	if allDead {
		return StatusDead
	}
	return StatusUnknown
}
