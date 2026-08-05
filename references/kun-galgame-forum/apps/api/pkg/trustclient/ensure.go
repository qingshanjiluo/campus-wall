package trustclient

import "context"

// Declarative subject-kind registration (trust onboarding §5). The forum's
// subject-kind universe is a property of the forum, so it is declared in code
// and self-reported to the trust registry on boot via this S2S face — the
// "register kinds" step becomes "edit a constant array", not "ask infra to run
// SQL / click the admin UI". site is derived server-side from the Basic
// credential (oauth_clients.catalog_site), never on the wire.
//
// Contract: kun-galgame-infra/docs/trust/onboarding.md §5 +
// docs/trust/openapi.yaml (POST /trust/subject-kinds/ensure).

// EnsureSubjectKindItem is one declared subject kind. The forum declares KEY
// ONLY: callback_url / callback_secret / notify_on_dismiss are optional and
// admin-configured infra-side, and the ensure face is SPARSE — a {key}-only
// item never clobbers those admin-set fields. Omitting them is intentional, not
// an oversight (matching the "resurrecting/reconfiguring a kind is an admin
// decision" philosophy).
type EnsureSubjectKindItem struct {
	Key string `json:"key"`
}

// EnsureSubjectKindResult is one per-kind convergence outcome, returned in
// request order. Result is one of created | updated | unchanged |
// deprecated_skipped — a deprecated kind is never resurrected by ensure.
type EnsureSubjectKindResult struct {
	Key    string `json:"key"`
	Result string `json:"result"`
}

// EnsureSubjectKinds declaratively registers the forum's subject-kind universe
// on its own site (Basic S2S). It is idempotent: the same slice twice converges
// to all-`unchanged`, so it is safe to fire on every boot. Returns
// ErrNotConfigured when the client is unwired (the caller warns and moves on).
func (c *Client) EnsureSubjectKinds(
	ctx context.Context, kinds []EnsureSubjectKindItem,
) ([]EnsureSubjectKindResult, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	body := struct {
		Kinds []EnsureSubjectKindItem `json:"kinds"`
	}{Kinds: kinds}
	var out struct {
		Results []EnsureSubjectKindResult `json:"results"`
	}
	if err := c.postJSON(ctx, "/api/v1/trust/subject-kinds/ensure", body, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}
