package trustclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Moderation faces (trust wave-1 forum integration): the synchronous pre-write
// word-list gate (/trust/check) and the async post-commit shadow scan
// (/trust/scan). Both are plain S2S Basic calls — kungal is a direct product
// site, so `site` is NEVER on the wire (the trust service derives it from
// oauth_clients.catalog_site). The caller (internal/trust/gate) owns the
// timeout + fail-open posture; these methods just answer or error.
//
// Contract: kun-galgame-infra/docs/trust/onboarding.md §3.1 / §3.2 +
// docs/trust/openapi.yaml. Reference impl: infra pkg/trustclient/client.go.

// CheckRequest mirrors the trust service's dto.CheckRequest. author_id is
// optional (accepted for future repeat-offender weighting, not consulted in
// v0). site is derived server-side from the S2S credential, never sent.
type CheckRequest struct {
	Text     string `json:"text"`
	AuthorID *int64 `json:"author_id,omitempty"`
}

// CheckResult mirrors the trust service's check `data`. Decision is one of
// allow|deny|hold (deny wins over hold); Matched is the normalized terms that
// tripped the gate ([] when none).
type CheckResult struct {
	Decision string   `json:"decision"`
	Matched  []string `json:"matched"`
}

// Check runs the synchronous Tier0 word-list gate. Returns ErrNotConfigured
// when the client has no base URL / credentials (the caller fails open). Any
// non-2xx / non-zero envelope code is a plain error the caller treats as allow.
func (c *Client) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var res CheckResult
	if err := c.postJSON(ctx, "/api/v1/trust/check", req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ScanRequest mirrors the trust service's dto.ScanRequest. subject_kind must be
// a kind registered for kungal's site (forum_topic / forum_reply are
// pre-registered); subject_id is the stringified stable id; text is the RAW
// body (capped/truncated at intake, never client-side). author_id is optional.
type ScanRequest struct {
	SubjectKind string `json:"subject_kind"`
	SubjectID   string `json:"subject_id"`
	Text        string `json:"text"`
	AuthorID    *int64 `json:"author_id,omitempty"`
}

// ScanResult mirrors the trust service's scan `data`. Truncated reports whether
// the text was clipped to the 8000-rune intake cap.
type ScanResult struct {
	ScanID    int64 `json:"scan_id"`
	Truncated bool  `json:"truncated"`
}

// Scan submits a content-scan event for async shadow-scoring. Returns
// ErrNotConfigured when the client is unwired. Best-effort: the caller logs and
// drops any error (a miss self-heals on the next edit).
func (c *Client) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	var res ScanResult
	if err := c.postJSON(ctx, "/api/v1/trust/scan", req, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// postJSON sends a Basic-authed JSON POST and decodes the house envelope's
// `data` into out. A non-2xx status or a non-zero envelope code is an error.
func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.basicAuth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("trustclient: decode %s response: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK || env.Code != 0 {
		return fmt.Errorf("trustclient: %s failed (status %d, code %d): %s", path, resp.StatusCode, env.Code, env.Message)
	}
	return json.Unmarshal(env.Data, out)
}
