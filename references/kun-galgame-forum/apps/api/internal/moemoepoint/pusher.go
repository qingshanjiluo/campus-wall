// Package moemoepoint is kungal's single emitter for moemoepoint (萌萌点)
// changes. moemoepoint is a site-wide unified balance whose single source of
// truth is the OAuth server (docs/oauth/06-moemoepoint.md). kungal no longer
// mutates the balance locally; every change goes through the Awarder, which
// calls OAuth's idempotent POST /users/:id/moemoepoint and then mirrors the
// returned AUTHORITATIVE balance into the local kungal_user_state.moemoepoint
// read-cache (which ranking / profile / /auth/me still read).
//
// Terminal state (mirrors kun-galgame-patch/apps/api):
//   - NO local `+=`. A local increment would double-count after the one-time
//     §6 cross-site merge migration. We only ever WRITE the value OAuth returns.
//   - Best-effort + non-blocking. Award runs in the background and never blocks
//     or fails the caller's core flow; a failed push only logs (soft karma —
//     a rarely-lost point is acceptable, and the stable idempotency key makes a
//     later retry / the cmd/sync-moemoepoint re-seed safe).
//
// DEPLOY ORDER: because the local += is gone, this must ship AFTER OAuth's
// moemoepoint endpoints are live AND the §6 balance merge has run; then run
// `go run ./cmd/sync-moemoepoint` to seed the local cache. Until then awards
// are best-effort no-ops against an unmigrated/unreachable OAuth.
package moemoepoint

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

// s2s reasons kungal may use (the OAuth enum is tiny; business detail rides on
// `ref`, not bespoke reasons). admin_grant/admin_deduct/migration are
// OAuth-reserved and rejected for service-to-service calls (06 §3.1).
const (
	ReasonDailyCheckin    = "daily_checkin"    // 每日签到 (+)
	ReasonLiked           = "liked"            // 内容被点赞 / 取消 (±)
	ReasonContentApproved = "content_approved" // 产出被采纳：发布/合并/奖励 (+)
	ReasonContentRemoved  = "content_removed"  // 产出被删/撤回，或消费扣除 (−)
)

// adjuster is the slice of *userclient.Client this package needs (interface
// for testability).
type adjuster interface {
	AdjustMoemoepoint(ctx context.Context, userID, delta int, reason, ref, idempotencyKey string) (userclient.MoemoepointResult, error)
}

// Awarder applies a moemoepoint change via OAuth (source of truth), then
// mirrors the returned authoritative balance into the local read-cache.
type Awarder struct {
	client  adjuster
	db      *gorm.DB
	timeout time.Duration
}

// NewAwarder wraps the OAuth client + the local DB (for the cache mirror). A
// nil client/db yields a no-op Awarder (safe before wiring / in tests).
func NewAwarder(client adjuster, db *gorm.DB) *Awarder {
	return &Awarder{client: client, db: db, timeout: 5 * time.Second}
}

// Award adjusts the user's unified balance on OAuth and syncs the local cache.
// Fire-and-forget: it returns immediately and does the OAuth call + cache
// mirror in the background. No-op on zero delta / missing client. NEVER does a
// local `+=`; it only writes the authoritative balance OAuth returns. Ideally
// call it AFTER the triggering DB work has committed.
func (a *Awarder) Award(userID, delta int, reason, ref, idempotencyKey string) {
	if a == nil || a.client == nil || userID <= 0 || delta == 0 {
		return
	}
	if delta > maxDelta {
		delta = maxDelta
	} else if delta < -maxDelta {
		delta = -maxDelta
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
		defer cancel()
		res, err := a.client.AdjustMoemoepoint(ctx, userID, delta, reason, ref, idempotencyKey)
		if err != nil {
			slog.Warn("moemoepoint award failed (best-effort, skipped)",
				"user_id", userID, "delta", delta, "reason", reason, "ref", ref, "err", err)
			return
		}
		// Mirror the authoritative balance into the local read-cache — never a
		// local +=. kungal_user_state is keyed by user_id.
		if a.db != nil {
			if err := a.db.WithContext(ctx).
				Exec(`UPDATE kungal_user_state SET moemoepoint = ? WHERE user_id = ?`, res.Balance, userID).Error; err != nil {
				slog.Warn("moemoepoint cache mirror failed",
					"user_id", userID, "balance", res.Balance, "err", err)
			}
		}
	}()
}

// AwardSync is the SYNCHRONOUS variant of Award for replayable cron paths
// (notably galgame-approve) that must know whether the OAuth call landed before
// advancing their durable cursor — otherwise a transient failure would be
// silently lost (the cron would move past the message and never retry). Same
// terminal-state contract as Award (NO local +=, mirrors the authoritative
// balance). Returns the OAuth error on failure so the caller can decide to
// retry (transient) or skip (permanent, see IsPermanentAwardError). The stable
// idempotency key makes a later retry a safe no-op (OAuth returns applied=false).
func (a *Awarder) AwardSync(userID, delta int, reason, ref, idempotencyKey string) error {
	if a == nil || a.client == nil || userID <= 0 || delta == 0 {
		return nil
	}
	if delta > maxDelta {
		delta = maxDelta
	} else if delta < -maxDelta {
		delta = -maxDelta
	}
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	res, err := a.client.AdjustMoemoepoint(ctx, userID, delta, reason, ref, idempotencyKey)
	if err != nil {
		return err
	}
	if a.db != nil {
		if err := a.db.WithContext(ctx).
			Exec(`UPDATE kungal_user_state SET moemoepoint = ? WHERE user_id = ?`, res.Balance, userID).Error; err != nil {
			slog.Warn("moemoepoint cache mirror failed",
				"user_id", userID, "balance", res.Balance, "err", err)
		}
	}
	return nil
}

// IsPermanentAwardError reports whether an AwardSync error is a definitive OAuth
// rejection (a business-code envelope: bad/reserved reason, user not found,
// idempotency-body conflict) rather than a transient network/timeout/5xx. A
// permanent error must NOT wedge a cron forever — the caller logs and skips it;
// a transient one should hold the cursor and retry.
func IsPermanentAwardError(err error) bool {
	if err == nil {
		return false
	}
	var oerr *userclient.OAuthError
	return errors.As(err, &oerr)
}

// maxDelta is the OAuth ±cap (06 §3.1); we clamp-guard locally too.
const maxDelta = 1_000_000

// ──────────────────────────────────────────
// Package-level default emitter (set once at app startup; nil = no-op).
// ──────────────────────────────────────────

var defaultAwarder *Awarder

// SetDefault installs the process-wide Awarder. Call once during app startup.
func SetDefault(a *Awarder) { defaultAwarder = a }

// Award emits via the package default (no-op if unset). See Awarder.Award.
func Award(userID, delta int, reason, ref, idempotencyKey string) {
	defaultAwarder.Award(userID, delta, reason, ref, idempotencyKey)
}

// AwardSync emits synchronously via the package default. See Awarder.AwardSync.
func AwardSync(userID, delta int, reason, ref, idempotencyKey string) error {
	return defaultAwarder.AwardSync(userID, delta, reason, ref, idempotencyKey)
}

// Key builds a kungal-namespaced idempotency key: "kungal:<part>:<part>:…".
// Use a STABLE set of parts for replayable events (cron rewards, daily
// check-in) so retries dedup.
func Key(parts ...string) string {
	return "kungal:" + strings.Join(parts, ":")
}

// KeyNonce is Key with a time-based nonce — for user-initiated, non-replayed
// actions (like/unlike toggles) where each action should record once and a
// false dedup is undesirable.
func KeyNonce(parts ...string) string {
	return Key(append(parts, strconv.FormatInt(time.Now().UnixNano(), 36))...)
}

// Ref builds a "kind:id" reference for the audit `ref` field (for reconciliation
// — a reward and its later reversal share the same ref).
func Ref(kind string, id int) string {
	return kind + ":" + strconv.Itoa(id)
}
