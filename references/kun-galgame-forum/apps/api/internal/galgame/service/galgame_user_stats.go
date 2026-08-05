package service

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/pkg/catalogclient"
)

// GalgameUserStats is the per-user contribution snapshot the profile page, the
// daily submission quota and creator eligibility all read.
//
// It used to come from one wiki endpoint that counted rows in the wiki's own
// tables. There is no such endpoint any more, and there should not be: the
// registry has no owner column and cannot have one, because a registry row
// outlives any account. So each number is derived from what the user actually
// DID — their claim transitions and their merged edits — which is the same
// definition the old counts were trying to express.
type GalgameUserStats struct {
	// Published is how many entries of this user's are live.
	Published int64 `json:"published"`
	// PublishedToday counts the claims this user first touched today, which is
	// what the daily submission quota is measured against.
	PublishedToday int `json:"published_today"`
	// Contributed is how many DISTINCT entries this user has landed an edit on.
	// Distinct, not merged-proposal count: "参与编辑的 Galgame" is a number of
	// games, and five edits to one game is one game.
	Contributed int `json:"contributed"`
	// MergedEdits is the raw merged-proposal count — the creator-eligibility
	// threshold, which is about volume of accepted work rather than breadth.
	MergedEdits int64 `json:"merged_edits"`
}

// GalgameUserStatsService derives the snapshot and the two profile lists that
// go with it.
type GalgameUserStatsService struct {
	catalog       *catalogclient.Client
	galgameClient *client.GalgameClient
}

func NewGalgameUserStatsService(
	catalog *catalogclient.Client,
	galgameClient *client.GalgameClient,
) *GalgameUserStatsService {
	return &GalgameUserStatsService{catalog: catalog, galgameClient: galgameClient}
}

const (
	// dailyClaimScan is how far back the "today" count looks. The list is
	// ordered by most recent activity, so today's rows are always at its head;
	// this is the ceiling on how many submissions one user can make in a day
	// before the count saturates, not a page size the caller pages through.
	dailyClaimScan = 100
	// contributedScan bounds the distinct-entry walk. A contributor past this
	// many merged edits is far beyond every threshold that reads the number.
	contributedScan = 200
)

// Stats builds the snapshot. Every field degrades independently: a face that
// is unreachable yields a zero for its own number and a warning, never an error
// for the whole profile — these are decorations on a page that has other
// reasons to render.
func (s *GalgameUserStatsService) Stats(ctx context.Context, uid int64) GalgameUserStats {
	var out GalgameUserStats
	if s.catalog == nil || !s.catalog.Configured() {
		return out
	}

	if page, err := s.catalog.UserClaims(ctx, uid, catalogclient.UserClaimFilter{
		Site: submissionSite, ClaimStates: []string{catalogclient.ClaimStateLive}, Limit: 1,
	}); err != nil {
		slog.Warn("user stats: 读取已发布计数失败", "uid", uid, "error", err)
	} else {
		// The total is counted under the same filter and is independent of the
		// cursor — which is exactly what lets one list face answer a statistic.
		out.Published = page.Total
	}

	if page, err := s.catalog.UserClaims(ctx, uid, catalogclient.UserClaimFilter{
		Site: submissionSite, Limit: dailyClaimScan,
	}); err != nil {
		slog.Warn("user stats: 读取今日投稿数失败", "uid", uid, "error", err)
	} else {
		out.PublishedToday = countToday(page.Items)
	}

	if total, err := s.catalog.CountEditProposals(ctx, catalogclient.EditProposalFilter{
		EntityType: catalogclient.EntityTypeWork, ProposerUID: uid, Status: "merged",
	}); err != nil {
		slog.Warn("user stats: 读取合并提案计数失败", "uid", uid, "error", err)
	} else {
		out.MergedEdits = total
	}

	if items, err := s.catalog.ListEditProposals(ctx, catalogclient.EditProposalFilter{
		EntityType: catalogclient.EntityTypeWork, ProposerUID: uid,
		Status: "merged", Limit: contributedScan,
	}); err != nil {
		slog.Warn("user stats: 读取贡献条目失败", "uid", uid, "error", err)
	} else {
		out.Contributed = len(distinctEntityIDs(items))
	}
	return out
}

// countToday counts the claims this user first acted on since local midnight.
//
// FirstActedAt is the user's OWN first transition on that claim, not the
// claim's last event — so a moderator approving yesterday's submission today
// does not consume today's quota, and re-touching an old submission does not
// either.
func countToday(items []catalogclient.UserClaimItem) int {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	count := 0
	for i := range items {
		if items[i].FirstActedAt.After(midnight) {
			count++
		}
	}
	return count
}

// distinctEntityIDs collapses a proposal list onto the entries it touched.
func distinctEntityIDs(items []catalogclient.EditProposal) []int64 {
	seen := make(map[int64]struct{}, len(items))
	out := make([]int64, 0, len(items))
	for i := range items {
		id := items[i].EntityID
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// maxClaimPageWalk bounds how deep the offset-paged profile tab may reach into
// a cursor-paged face. The two paginations do not compose — a cursor page is
// only reachable by walking to it — so the walk is capped rather than allowed
// to turn a deep-link into an unbounded fan-out.
const maxClaimPageWalk = 20

// PublishedGIDs is the "已发布的 Galgame" tab: the entries this user has live,
// most recent activity first, brought home to kungal ids.
//
// The offset page is walked over the cursor face rather than translated: a
// cursor page is only reachable through the ones before it, and pretending
// otherwise is how a list starts skipping rows while it is being written to.
func (s *GalgameUserStatsService) PublishedGIDs(
	ctx context.Context, uid int64, page, limit int,
) ([]int, int64, error) {
	return s.claimGIDs(ctx, uid, []string{catalogclient.ClaimStateLive}, page, limit)
}

func (s *GalgameUserStatsService) claimGIDs(
	ctx context.Context, uid int64, states []string, page, limit int,
) ([]int, int64, error) {
	if page < 1 {
		page = 1
	}
	if page > maxClaimPageWalk {
		return []int{}, 0, nil
	}
	var (
		before int64
		total  int64
		items  []catalogclient.UserClaimItem
	)
	for i := 0; i < page; i++ {
		p, err := s.catalog.UserClaims(ctx, uid, catalogclient.UserClaimFilter{
			Site: submissionSite, ClaimStates: states, Before: before, Limit: limit,
		})
		if err != nil {
			return nil, 0, err
		}
		total, items, before = p.Total, p.Items, p.NextBefore
		if before == 0 {
			// Tail reached: a later page is empty, not an error.
			if i < page-1 {
				return []int{}, total, nil
			}
			break
		}
	}
	gids := make([]int, 0, len(items))
	for i := range items {
		// The claim's product id IS the gid — it is the id kungal told the
		// registry to record. No bridge lookup needed on this lane.
		if id := items[i].ProductWorkID; id != nil && *id > 0 {
			gids = append(gids, int(*id))
		}
	}
	return gids, total, nil
}

// ContributedGIDs is the "参与编辑的 Galgame" tab: the entries this user has
// landed an edit on. Unlike the claim lane these are REGISTRY ids, so they go
// back through the bridge — an entity_id used as a gid would link to a
// different game without erroring.
func (s *GalgameUserStatsService) ContributedGIDs(ctx context.Context, uid int64) ([]int, error) {
	items, err := s.catalog.ListEditProposals(ctx, catalogclient.EditProposalFilter{
		EntityType: catalogclient.EntityTypeWork, ProposerUID: uid,
		Status: "merged", Limit: contributedScan,
	})
	if err != nil {
		return nil, err
	}
	workIDs := distinctEntityIDs(items)
	gidByWork, appErr := s.galgameClient.GIDsByCatalogIDs(ctx, workIDs)
	if appErr != nil {
		return nil, appErr
	}
	// Order follows the proposals (most recent first); works kungal no longer
	// claims simply drop out.
	gids := make([]int, 0, len(workIDs))
	for _, id := range workIDs {
		if gid, ok := gidByWork[id]; ok {
			gids = append(gids, gid)
		}
	}
	return gids, nil
}
