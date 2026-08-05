package service

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/pkg/catalogclient"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// GalgameEditRevisionSync mirrors editing-engine revisions into the local
// galgame_activity table so the forum activity timeline can render "user X
// edited galgame Y" without querying a remote service at page-render time.
//
// Source (wave 156 N3): the catalog editing engine's global revision cursor
// feed, narrowed to entity_type=galgame.game. It replaces the wiki's
// /galgame/revisions/recent feed, which had been under-reporting badly: the
// engine has been the sole author of galgame field edits since E3a, and in the
// same window the wiki feed delivered 312 events the engine recorded 2,803 of.
// A supply A/B before the switch confirmed containment in the direction that
// matters — every (galgame_id, revision) pair the wiki feed ever delivered
// appears in the engine feed under the same (entity_id, seq), so nothing the
// timeline already shows loses its diff link.
//
// Delivery model is unchanged from the feed it replaces: a durable Redis cursor
// advances only past events whose local upsert succeeded, and the upsert is
// idempotent (ON CONFLICT on edit_revision_id), so at-least-once delivery is
// effectively exactly-once. A transient failure holds the cursor.
type GalgameEditRevisionSync struct {
	catalog *catalogclient.Client
	// galgameClient brings a revision's entity id home. The feed is keyed on
	// REGISTRY work ids and galgame_activity.galgame_id is a gid; the two spaces
	// overlap, so writing one into the other attaches the edit card to a
	// different game and raises nothing.
	galgameClient *client.GalgameClient
	db            *gorm.DB
	rdb           *redis.Client
	// batch is the feed page size. A field rather than a bare constant so the
	// paging tests can drive several pages without a 1000-row fixture; nothing
	// in production ever sets it to anything but editRevisionBatch.
	batch int
}

func NewGalgameEditRevisionSync(
	catalog *catalogclient.Client,
	galgameClient *client.GalgameClient,
	db *gorm.DB,
	rdb *redis.Client,
) *GalgameEditRevisionSync {
	return &GalgameEditRevisionSync{
		catalog: catalog, galgameClient: galgameClient,
		db: db, rdb: rdb, batch: editRevisionBatch,
	}
}

const (
	// editRevisionCursorKey holds the last-processed engine revision id. A NEW
	// key, not the wiki cron's `wiki:rev:cron:since`: the two feeds number their
	// rows in different namespaces, so inheriting the old cursor would resume at
	// a meaningless offset.
	editRevisionCursorKey = "catalog:rev:cron:since"
	// editRevisionEntityType narrows the global feed to the registry's works —
	// the type kungal's galgame edits now target.
	editRevisionEntityType = catalogclient.EntityTypeWork
	// editRevisionBatch is the page size (the feed caps at 1000).
	editRevisionBatch = 1000
	// editRevisionMaxPages guards against a runaway feed.
	editRevisionMaxPages = 50
)

// Run executes one sync cycle. Cheap when there is nothing new (one GET that
// returns an empty page).
func (s *GalgameEditRevisionSync) Run() {
	if s.catalog == nil || !s.catalog.Configured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	since, seeded, err := s.readCursor(ctx)
	if err != nil {
		// Unknown Redis state: skip this tick rather than risk re-seeding at the
		// head, which would silently drop every edit since the real cursor.
		slog.Warn("读取 catalog 修订游标失败 (本轮跳过)", "error", err)
		return
	}
	if !seeded {
		// First run after the switch: seed the cursor at the feed's head instead
		// of ingesting from 0. The engine's table holds the whole imported edit
		// history (12,581 rows, 9,694 of them entity CREATIONS), and replaying it
		// would rewrite the timeline's past rather than continue it. The events
		// the old feed already delivered are in the table under their wiki ids;
		// backfilling the ones it missed is a deliberate one-off, not a side
		// effect of a deploy.
		head, err := s.feedHead(ctx)
		if err != nil {
			slog.Warn("catalog 修订 feed 定位队首失败, 下一轮重试", "error", err)
			return
		}
		s.writeCursor(ctx, head)
		slog.Info("catalog 修订同步已初始化游标 (不回填历史)", "head", head)
		return
	}

	maxSeen := since
	startedFrom := since
	applied := 0

	for pages := 1; pages <= editRevisionMaxPages; pages++ {
		page, err := s.catalog.EditRevisionsSince(ctx, maxSeen, s.batch, editRevisionEntityType)
		if err != nil {
			// Don't advance the cursor — the next tick retries from here.
			slog.Warn("catalog 修订 feed 拉取失败", "error", err, "since", maxSeen)
			break
		}
		if len(page.Items) == 0 {
			break
		}

		gidByWork := s.gidsFor(ctx, page.Items)

		holding := false
		for i := range page.Items {
			rev := &page.Items[i]
			if err := s.upsert(rev, gidByWork[rev.EntityID]); err != nil {
				// Transient DB failure: hold the cursor BEFORE this id so the
				// next tick re-attempts (idempotent ON CONFLICT).
				slog.Warn("catalog 修订入库失败, 持有游标重试", "rev_id", rev.ID, "error", err)
				holding = true
				break
			}
			if rev.ID > maxSeen {
				maxSeen = rev.ID
			}
			applied++
		}
		if holding || len(page.Items) < s.batch {
			break
		}
	}

	if maxSeen > startedFrom {
		s.writeCursor(ctx, maxSeen)
		slog.Info("catalog 修订同步完成", "from", startedFrom, "to", maxSeen, "applied", applied)
	}
}

// feedHead walks the feed once WITHOUT applying anything, to learn its current
// last id. Used only to seed a fresh cursor.
func (s *GalgameEditRevisionSync) feedHead(ctx context.Context) (int64, error) {
	var head int64
	for pages := 1; pages <= editRevisionMaxPages; pages++ {
		page, err := s.catalog.EditRevisionsSince(ctx, head, s.batch, editRevisionEntityType)
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

// isTimelineEdit reports whether an engine revision is an EDIT a timeline
// should show. A `created` revision is the entity's birth — the registry
// minting the row, with every field "changed" from nothing — and the forum
// already announces a new galgame through its own creation activity; letting it
// through would put a card reading "user X edited Y" on a game nobody has
// edited yet. Merged, direct and reverted are all human content changes.
func isTimelineEdit(action int16) bool {
	return action != catalogclient.EditActionCreated
}

// upsert writes one engine revision into galgame_activity.
//
// `seq` goes into wiki_revision_number because it IS the per-galgame revision
// number the edit card's diff URL takes as :rev — so the card renders an
// engine-fed row exactly as it rendered a wiki-fed one. wiki_revision_id stays
// NULL: the row has no wiki-side identity.
//
// Entity CREATIONS are skipped (see isTimelineEdit).
func (s *GalgameEditRevisionSync) upsert(rev *catalogclient.EditRevisionFeedItem, gid int) error {
	if !isTimelineEdit(rev.Action) {
		return nil
	}
	if gid == 0 {
		// An edit to a work kungal does not claim. Not an error and not a retry:
		// the registry is shared, and another product's entries have no place on
		// this timeline. Advancing past it is correct.
		return nil
	}
	return s.db.Exec(`
		INSERT INTO galgame_activity (edit_revision_id, wiki_revision_number, galgame_id, user_id, type, created)
		VALUES (?, ?, ?, ?, 'GALGAME_EDIT', ?)
		ON CONFLICT (edit_revision_id) DO NOTHING
	`, rev.ID, rev.Seq, gid, rev.ActorUID, rev.CreatedAt).Error
}

// gidsFor translates a page's entity ids in ONE call. A failure yields an empty
// map, which makes every row of the page a skip — and because the cursor only
// advances past rows that were applied, a page whose translation failed is
// retried rather than silently dropped.
func (s *GalgameEditRevisionSync) gidsFor(
	ctx context.Context,
	items []catalogclient.EditRevisionFeedItem,
) map[int64]int {
	workIDs := make([]int64, 0, len(items))
	seen := make(map[int64]bool, len(items))
	for i := range items {
		if id := items[i].EntityID; !seen[id] {
			seen[id] = true
			workIDs = append(workIDs, id)
		}
	}
	if s.galgameClient == nil {
		return map[int64]int{}
	}
	gids, appErr := s.galgameClient.GIDsByCatalogIDs(ctx, workIDs)
	if appErr != nil {
		slog.Warn("catalog 修订 work id → gid 失败", "error", appErr)
		return map[int64]int{}
	}
	return gids
}

// readCursor returns the stored cursor, whether one existed, and any Redis
// failure. "Missing" and "unreadable" must stay distinguishable: the first means
// "never synced" (seed at head), the second must NOT reseed.
func (s *GalgameEditRevisionSync) readCursor(ctx context.Context) (int64, bool, error) {
	v, err := s.rdb.Get(ctx, editRevisionCursorKey).Int64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (s *GalgameEditRevisionSync) writeCursor(ctx context.Context, id int64) {
	if err := s.rdb.Set(ctx, editRevisionCursorKey, id, 0).Err(); err != nil {
		slog.Warn("写入 catalog 修订游标失败", "id", id, "error", err)
	}
}
