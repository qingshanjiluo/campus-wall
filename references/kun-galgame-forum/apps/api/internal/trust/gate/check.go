// Package gate wires kungal's forum write paths (topic + reply create/edit) to
// the infra Trust & Safety moderation faces: the SYNCHRONOUS pre-write word-list
// gate (check) and the ASYNC post-commit shadow scan (scan). Both are entirely
// fail-open / best-effort — a down or slow trust service must never block or
// delay a post (onboarding §6 red lines). Reference impl (semantics mirrored
// line-for-line): kun-galgame-infra internal/platform/community/service/{check,scan}.go.
package gate

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/pkg/trustclient"
)

// checkTimeout bounds the synchronous pre-write word-list check. It sits on the
// create/edit request path, so it is tightly bounded: a slower or erroring trust
// service must never stall a post. On timeout the check fails OPEN — the write
// proceeds (onboarding §3.1 caller discipline).
const checkTimeout = 500 * time.Millisecond

// Check decisions on the wire (mirror trust service.Decision*). deny wins over
// hold; allow is the clean path.
const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
	DecisionHold  = "hold"
)

// Checker is the trust check-face client, abstracted so tests fake it.
// *trustclient.Client satisfies it directly.
type Checker interface {
	Check(ctx context.Context, req trustclient.CheckRequest) (*trustclient.CheckResult, error)
}

// CheckService runs the synchronous Tier0 word-list gate BEFORE a forum write
// commits. It is entirely fail-open: a nil checker, or any error/timeout,
// resolves to allow (with an slog.Warn) so a down/slow trust service never
// blocks posting. A nil *CheckService is itself a valid "gate off" value —
// Decision is nil-receiver safe.
type CheckService struct {
	ck Checker
}

// NewCheckService builds the gate. ck nil = the gate is off (every Decision is
// allow); app.go passes a non-nil checker only when KUN_TRUST_CHECK_ENABLED is
// set AND the trust client is configured.
func NewCheckService(ck Checker) *CheckService {
	return &CheckService{ck: ck}
}

// Enabled reports whether the gate will actually dial.
func (s *CheckService) Enabled() bool { return s != nil && s.ck != nil }

// Decision runs the gate for one piece of content and returns
// DecisionAllow/DecisionDeny/DecisionHold plus the matched terms (for the hold
// audit log). A disabled gate or any checker failure is a fail-open allow. The
// 500ms timeout is applied here — the pkg client stays timeout-agnostic. The
// HTTP call never runs inside a DB transaction: callers invoke Decision strictly
// before opening their tx.
func (s *CheckService) Decision(ctx context.Context, text string, authorID *int64) (decision string, matched []string) {
	if !s.Enabled() {
		return DecisionAllow, nil
	}
	cctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	res, err := s.ck.Check(cctx, trustclient.CheckRequest{Text: text, AuthorID: authorID})
	if err != nil {
		slog.Warn("trust check fail-open", "err", err)
		return DecisionAllow, nil
	}
	switch res.Decision {
	case DecisionDeny:
		return DecisionDeny, res.Matched
	case DecisionHold:
		return DecisionHold, res.Matched
	default:
		return DecisionAllow, res.Matched
	}
}
