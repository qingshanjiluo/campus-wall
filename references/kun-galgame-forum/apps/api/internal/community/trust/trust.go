// Package trust wires kungal (a consuming site) into the community primitive's
// trust engine: the starter Boost declared from the user's roles + account age
// at login (charter ruling 6). Best-effort and never blocks the login request;
// the primitive owns promotion/demotion. Mirrors the letmoe consumer's
// internal/community/trust, adapted to kungal's local account store.
//
// Gating: the whole reporter is inert unless the S2S community client is
// configured — so on a dev box without a community service no Boost ever fires.
package trust

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/role"
)

// veteranAge is the account-age threshold above which an established kungal
// account skips the TL0 cold start (charter ruling 6: ≥30d → veteran).
const veteranAge = 30 * 24 * time.Hour

// boostOnceTTL bounds the per-user once-guard; a boost is a fixed declaration,
// no need to re-send within the window (mirrors letmoe's 90d).
const boostOnceTTL = 90 * 24 * time.Hour

// Reporter declares starter boosts to the community primitive.
type Reporter struct {
	cli *communityclient.Client
	rdb *redis.Client
	db  *gorm.DB
}

// New builds the reporter. When the client is unconfigured every call is a
// no-op regardless of rdb/db.
func New(cli *communityclient.Client, rdb *redis.Client, db *gorm.DB) *Reporter {
	return &Reporter{cli: cli, rdb: rdb, db: db}
}

// active reports whether the reporter should do anything: the S2S community
// client is configured.
func (r *Reporter) active() bool {
	return r != nil && r.cli != nil && r.cli.Configured()
}

// Boost declares the starter boost for a user from their roles + account age, at
// most once per user (a Redis guard; the server is idempotent and only ever
// raises the floor). Non-blocking: it returns immediately and does all work —
// including the account-age DB read — in the background, so it never slows the
// login path. A no-op when the reporter is inert (client unconfigured).
func (r *Reporter) Boost(userID int, roles []string) {
	if !r.active() || userID <= 0 {
		return
	}
	go r.declare(userID, roles)
}

func (r *Reporter) declare(userID int, roles []string) {
	ctx := context.Background()
	// staff (moderator/admin/ren) skips the TL0 cold start; otherwise an
	// established local account (≥30d) is a veteran. kungal declares no creator
	// boost (charter ruling 6 lists only staff + veteran for this site).
	boost := boostForRoles(roles)
	if boost == communityclient.BoostNone && r.isVeteran(ctx, userID) {
		boost = communityclient.BoostVeteran
	}
	if boost == communityclient.BoostNone {
		return
	}

	if r.rdb != nil {
		// Once per user — a fixed declaration, no need to re-send.
		set, err := r.rdb.SetNX(ctx, "kungal:community:boosted:"+strconv.Itoa(userID), boost, boostOnceTTL).Result()
		if err == nil && !set {
			return
		}
	}
	if _, err := r.cli.Boost(ctx, communityclient.SetBoostRequest{UserID: int64(userID), Boost: boost}); err != nil {
		slog.Warn("community trust boost declare failed", "user_id", userID, "boost", boost, "error", err)
	}
}

// boostForRoles maps a role set to a starter boost: staff (moderator/admin/ren)
// → the staff floor, everything else → none. kungal declares no creator boost
// (charter ruling 6 lists only staff + veteran for this site). Pure.
func boostForRoles(roles []string) int32 {
	if role.CanModerate(roles) {
		return communityclient.BoostStaff
	}
	return communityclient.BoostNone
}

// isVeteranAge reports whether an account created at `created` is at least
// veteranAge old as of `now`. A zero timestamp (no row) is never a veteran. Pure.
func isVeteranAge(created, now time.Time) bool {
	return !created.IsZero() && now.Sub(created) >= veteranAge
}

// isVeteran reports whether the local kungal account is at least veteranAge old.
// The account store is kungal_user_state.created (the "等价注册时间" — first seen
// on this site; charter ruling 6). A missing/zero row or a DB error reads as
// not-veteran (fail-safe: a boost only ever raises the floor, so a missed
// veteran just climbs from TL0 normally).
func (r *Reporter) isVeteran(ctx context.Context, userID int) bool {
	if r.db == nil {
		return false
	}
	var created time.Time
	err := r.db.WithContext(ctx).
		Raw("SELECT created FROM kungal_user_state WHERE user_id = ?", userID).
		Scan(&created).Error
	if err != nil {
		return false
	}
	return isVeteranAge(created, time.Now())
}
