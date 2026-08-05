package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/pkg/catalogclient"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// GalgameClaimEventSync ingests the registry's claim-transition feed and
// applies kungal's side of each transition. It replaces GalgameMessageSync,
// whose upstream (the wiki `galgame_message` table and its /messages/feed)
// retires with the wiki tables.
//
// The two feeds are not translations of each other, and the differences are
// what this file is mostly about:
//
//   - The wiki delivered TYPED messages (approved / declined / banned /
//     unbanned). The registry delivers TRANSITIONS (from_state, to_state).
//     A type word is a claim about intent; a transition is a fact, and one
//     transition can arrive by more than one route — `live` is reached both by
//     a reviewer approving a submission and by the owning site publishing its
//     own draft. kungal's local side effect keys off the DESTINATION state,
//     which is the only thing that determines whether a stub should exist.
//   - The wiki named the beneficiary of the +3 (`target_user_id`). The
//     registry names the ACTOR, which on an approval is the reviewer. See
//     submitterOf below — attributing the award is the one place where the
//     two feeds are not equivalent.
//   - The id spaces are disjoint and both are small integers, so the cursor
//     key AND the moemoepoint idempotency key both move into fresh namespaces.
//     Reusing either would silently mistake a wiki message for a claim event.
//
// Delivery semantics are unchanged and deliberately so: at-least-once plus
// idempotent side effects. The durable cursor advances past an event only once
// its effects have landed; a transient failure holds the cursor and the next
// tick retries; a permanent rejection is logged and skipped so one poison row
// cannot wedge the feed.
type GalgameClaimEventSync struct {
	catalog     *catalogclient.Client
	galgameRepo *repository.GalgameRepository
	rdb         *redis.Client
	// batch is the feed page size. A field rather than a bare constant so the
	// paging tests can drive several pages without a 1000-row fixture; nothing
	// in production ever sets it to anything but claimFeedBatch.
	batch int
}

func NewGalgameClaimEventSync(
	catalog *catalogclient.Client,
	galgameRepo *repository.GalgameRepository,
	rdb *redis.Client,
) *GalgameClaimEventSync {
	return &GalgameClaimEventSync{
		catalog: catalog, galgameRepo: galgameRepo, rdb: rdb, batch: claimFeedBatch,
	}
}

const (
	// claimCursorKey is the durable cursor. NEW key: the wiki cursor
	// (`wiki:msg:cron:since`) holds a wiki message id, and pointing this feed
	// at it would start the run somewhere arbitrary in a different id space.
	// The old key is left untouched so a rollback resumes where it left off.
	claimCursorKey = "catalog:claim:cron:since"

	// claimSubmitterKeyPrefix remembers who submitted a work for review, keyed
	// by registry work id. See submitterOf.
	claimSubmitterKeyPrefix = "catalog:claim:submitter:"

	// claimSubmitterTTL bounds that memory. A submission that sits unreviewed
	// longer than this loses its attribution rather than its approval — the
	// entry still publishes, only the +3 is skipped (and logged).
	claimSubmitterTTL = 90 * 24 * time.Hour

	claimFeedBatch    = 1000
	claimMaxPagesRun  = 50
	claimFeedPageWait = 5 * time.Minute
)

// Run executes one sync cycle.
func (s *GalgameClaimEventSync) Run() {
	if s.catalog == nil || !s.catalog.Configured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), claimFeedPageWait)
	defer cancel()

	since, seeded, err := s.readCursor(ctx)
	if err != nil {
		// Unknown Redis state: skip this tick rather than risk re-seeding at the
		// head, which would silently drop every transition since the real cursor.
		slog.Warn("读取 claim 事件游标失败 (本轮跳过)", "error", err)
		return
	}
	if !seeded {
		// First run after the switch: position at the feed's head, do not
		// ingest from 0.
		//
		// This is not an optimisation, it is a correctness requirement. The
		// re-site step mints ONE backfill event per existing claim so the
		// per-user face and the review queue work on day one — tens of
		// thousands of rows, most of them to_state=live. Consuming them would
		// create a local stub for every entry in the registry, and the local
		// table is the anchor kungal's browse list is built from: the browse
		// population would change by an order of magnitude as a side effect of
		// a deploy. Those claims are already reflected locally; the feed exists
		// to carry what happens NEXT.
		head, err := s.feedHead(ctx)
		if err != nil {
			slog.Warn("catalog claim feed 定位队首失败, 下一轮重试", "error", err)
			return
		}
		s.writeCursor(ctx, head)
		slog.Info("catalog claim 同步已初始化游标 (不回填历史)", "head", head)
		return
	}

	startedFrom, maxSeen := since, since
	for pages := 1; pages <= claimMaxPagesRun; pages++ {
		page, err := s.catalog.ClaimEventsSince(ctx, maxSeen, s.batch, claimSite)
		if err != nil {
			slog.Warn("catalog claim feed 拉取失败", "error", err, "since", maxSeen)
			break
		}
		if len(page.Items) == 0 {
			break
		}
		holding := false
		for i := range page.Items {
			if s.apply(ctx, &page.Items[i]) {
				// Transient failure: hold the cursor BEFORE this event so the
				// next tick re-applies it (every effect is idempotent).
				holding = true
				break
			}
			if page.Items[i].ID > maxSeen {
				maxSeen = page.Items[i].ID
			}
		}
		if holding || len(page.Items) < s.batch {
			break
		}
	}

	if maxSeen > startedFrom {
		s.writeCursor(ctx, maxSeen)
		slog.Info("catalog claim 同步完成", "from", startedFrom, "to", maxSeen)
	}
}

// feedHead walks the feed once WITHOUT applying anything, to learn its current
// last id. Used only to seed a fresh cursor.
func (s *GalgameClaimEventSync) feedHead(ctx context.Context) (int64, error) {
	var head int64
	for pages := 1; pages <= claimMaxPagesRun; pages++ {
		page, err := s.catalog.ClaimEventsSince(ctx, head, s.batch, claimSite)
		if err != nil {
			return 0, err
		}
		if len(page.Items) == 0 {
			break
		}
		head = page.Items[len(page.Items)-1].ID
		if len(page.Items) < s.batch {
			break
		}
	}
	return head, nil
}

// claimSite is the tenant whose transitions kungal consumes. Narrowing the
// feed server-side matters here in a way it did not on the wiki feed: the
// registry is multi-tenant and an unfiltered feed would hand kungal another
// product's submissions to create local stubs for.
//
// It is the CLAIM site, so it is the claim-site constant — not the editing
// engine's tenant key, which is a different axis that merely spells the same
// today (doc 156 P1).
const claimSite = client.ClaimSiteKungal

// claimEffect is what one transition asks kungal to do locally.
type claimEffect int

const (
	claimEffectNone claimEffect = iota
	claimEffectSeedStub
	claimEffectDropStub
	claimEffectRememberSubmitter
	claimEffectUnknownState
)

// effectOf maps a transition onto its local effect. It keys off the
// DESTINATION state alone, because that — not the route taken to it — is what
// decides whether kungal needs a row.
//
// The two states that do nothing are deliberate, not omissions:
//
//   - declined: the entry was never publicly visible, so there is nothing to
//     take down.
//   - draft: a withdrawal from live keeps the stub. The entry can be published
//     again, and the stub carries the likes, comments and ratings it collected
//     while it was up — dropping it would destroy user content to reflect a
//     reversible editorial state.
func effectOf(ev *catalogclient.ClaimEventFeedItem) claimEffect {
	// No product-side anchor = nothing local to do. A registry-only claim (or
	// one whose anchor a merge cleared) has no row in kungal's key space, and
	// inventing one from the work id would attach another game's stats — the
	// two id spaces overlap (doc 106 R3).
	if ev.ProductWorkID == nil || *ev.ProductWorkID <= 0 {
		return claimEffectNone
	}
	switch ev.ToState {
	case catalogclient.ClaimStateLive:
		return claimEffectSeedStub
	case catalogclient.ClaimStateHidden:
		return claimEffectDropStub
	case catalogclient.ClaimStatePending:
		return claimEffectRememberSubmitter
	case catalogclient.ClaimStateDraft, catalogclient.ClaimStateDeclined:
		return claimEffectNone
	default:
		return claimEffectUnknownState
	}
}

// isApproval reports whether a transition is a reviewer approving a submission
// — the one route into `live` that carries the +3. The other route (an owner
// publishing their own draft) is rewarded in the request path instead, so
// matching on the destination alone would pay twice.
func isApproval(ev *catalogclient.ClaimEventFeedItem) bool {
	return ev.ToState == catalogclient.ClaimStateLive &&
		ev.FromState != nil && *ev.FromState == catalogclient.ClaimStatePending
}

// claimantOf resolves who a freshly-live entry credits as its creator (the
// author chip on the kungal card). The two routes into `live` name different
// people: an approval's actor is the REVIEWER, so the claimant is the
// remembered submitter — the same memo the +3 award reads — while every other
// route's actor moved their own claim. 0 = unknown (memo expired or a
// system-minted event); the stub then keeps a NULL creator and renders no
// author chip, never a wrong one.
func (s *GalgameClaimEventSync) claimantOf(ctx context.Context, ev *catalogclient.ClaimEventFeedItem) int {
	if isApproval(ev) {
		return s.submitterOf(ctx, ev.WorkID)
	}
	return int(ev.ActorUID)
}

// apply performs one transition's local side effects. retry=true means a
// TRANSIENT failure — the caller holds the cursor and this event is re-applied
// next tick, which is safe because every effect below is idempotent.
func (s *GalgameClaimEventSync) apply(ctx context.Context, ev *catalogclient.ClaimEventFeedItem) (retry bool) {
	switch effectOf(ev) {
	case claimEffectSeedStub:
		// live is the moment an entry becomes publicly listable — seed the stub
		// so the browse list has an anchor. Idempotent.
		gid := int(*ev.ProductWorkID)
		creator := s.claimantOf(ctx, ev)
		if err := s.galgameRepo.DB().Transaction(func(tx *gorm.DB) error {
			if err := s.galgameRepo.CreateLocalStub(tx, gid); err != nil {
				return err
			}
			if creator > 0 {
				return s.galgameRepo.SetCreatorIfUnset(tx, gid, creator)
			}
			return nil
		}); err != nil {
			slog.Warn("claim live: 创建本地 stub 失败, 将重试", "event", ev.ID, "gid", gid, "error", err)
			return true
		}
		if isApproval(ev) {
			return s.awardApproval(ctx, ev, gid)
		}
	case claimEffectDropStub:
		// The owning product took the entry down. Drop the stub; CASCADE takes
		// the interactions with it, which is what tidies up the orphans a like
		// or comment lazily created while the entry was still visible.
		s.galgameRepo.DeleteLocalStub(int(*ev.ProductWorkID))
	case claimEffectRememberSubmitter:
		// Remember who asked for review — the approval event will not say.
		s.rememberSubmitter(ctx, ev)
	case claimEffectUnknownState:
		slog.Warn("收到未识别的 claim 目标状态, 跳过", "state", ev.ToState, "event", ev.ID)
	}
	return false
}

// awardApproval grants the +3 for an approved submission, to whoever earned
// it, or says loudly why it could not.
//
// The wiki feed named the beneficiary outright (`target_user_id`). The registry
// names the ACTOR, and on an approval the actor is the reviewer — paying them
// would invert the reward. The beneficiary is the submitter, and the only place
// that identity appears on this feed is the earlier draft/declined -> pending
// event, so it comes from what the cron itself saw on the way in.
//
// The gap that leaves is bounded and one-directional: a submission whose
// `pending` event predates this cursor (or fell out of the memo's TTL) publishes
// normally and loses only its +3, which is logged at error level. It cannot
// pay the wrong person and it cannot block a review.
func (s *GalgameClaimEventSync) awardApproval(ctx context.Context, ev *catalogclient.ClaimEventFeedItem, gid int) (retry bool) {
	uid := s.submitterOf(ctx, ev.WorkID)
	if uid <= 0 {
		// Loud, and only ever a missing REWARD: the entry is live either way.
		slog.Error("claim approve: 无法归属投稿人, 跳过发奖",
			"event", ev.ID, "work", ev.WorkID, "gid", gid)
		return false
	}
	// STABLE per-event key so a cron replay is a no-op. The event id namespace
	// is new, so the `claim_approved` prefix is new too — `wiki_approved` holds
	// wiki message ids, and the two sequences overlap numerically.
	if err := moemoepoint.AwardSync(uid, constants.RewardCreateGalgame,
		moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame", gid),
		moemoepoint.Key("claim_approved", strconv.FormatInt(ev.ID, 10))); err != nil {
		if moemoepoint.IsPermanentAwardError(err) {
			slog.Error("claim approve: 发奖被 OAuth 永久拒绝, 跳过该事件",
				"event", ev.ID, "target", uid, "error", err)
			return false
		}
		slog.Warn("claim approve: 发奖瞬时失败, 将重试", "event", ev.ID, "target", uid, "error", err)
		return true
	}
	return false
}

// rememberSubmitter records the actor of a draft/declined -> pending move.
// Best-effort: losing the note costs the submitter's +3, never the review.
func (s *GalgameClaimEventSync) rememberSubmitter(ctx context.Context, ev *catalogclient.ClaimEventFeedItem) {
	if ev.ActorUID <= 0 {
		return
	}
	key := claimSubmitterKeyPrefix + strconv.FormatInt(ev.WorkID, 10)
	if err := s.rdb.Set(ctx, key, ev.ActorUID, claimSubmitterTTL).Err(); err != nil {
		slog.Warn("记录投稿人失败", "work", ev.WorkID, "error", err)
	}
}

func (s *GalgameClaimEventSync) submitterOf(ctx context.Context, workID int64) int {
	v, err := s.rdb.Get(ctx, claimSubmitterKeyPrefix+strconv.FormatInt(workID, 10)).Int()
	if err != nil {
		return 0
	}
	return v
}

// readCursor distinguishes "never seeded" (redis.Nil) from "unreadable", which
// a single int64 cannot express and which must not be conflated: the first
// means seed at the head, the second means do nothing at all.
func (s *GalgameClaimEventSync) readCursor(ctx context.Context) (since int64, seeded bool, err error) {
	v, err := s.rdb.Get(ctx, claimCursorKey).Int64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (s *GalgameClaimEventSync) writeCursor(ctx context.Context, id int64) {
	if err := s.rdb.Set(ctx, claimCursorKey, id, 0).Err(); err != nil {
		slog.Warn("写入 claim 事件游标失败", "id", id, "error", err)
	}
}
