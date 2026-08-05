package service

import (
	"log/slog"
	"math"
	"time"

	"gorm.io/gorm"
)

// The activity feed is trigger-materialized from the LEGACY comment tables
// (migration 034): community-backed comments live in another database, so those
// AFTER INSERT/DELETE triggers can never fire for them. These helpers replay the
// exact trigger projection through the same feed_upsert()/feed_delete() SQL
// functions, shared by the galgame comment path and the three resource-comment
// paths. All best-effort: the feed is a derived surface and must never fail a
// write.

// feedTypeGalgameComment is the galgame_comment trigger's feed type. Its
// projection carries the galgame id in the gid slot — the resource-comment
// triggers pass 0 there.
const feedTypeGalgameComment = "GALGAME_COMMENT_CREATION"

// feedParityUpsert replays feed_upsert(...) for a community post. source_id is
// an int32 column; ids beyond it are skipped + logged (a latent cap, not a live
// risk).
func feedParityUpsert(db *gorm.DB, feedType string, postID int64, userID, gid int, content, link string, nsfw bool, createdAt string) {
	if postID > math.MaxInt32 {
		slog.Warn("feed_activity source_id overflow — community post id exceeds int32; feed parity row skipped", "post_id", postID, "type", feedType)
		return
	}
	created, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		created = time.Now()
	}
	if err := db.Exec("SELECT feed_upsert(?, ?, ?, ?, ?, ?, ?, ?)",
		feedType, postID, userID, gid, truncate(content, 100), link, nsfw, created).Error; err != nil {
		slog.Warn("feed_activity parity upsert failed (best-effort)", "post_id", postID, "type", feedType, "error", err)
	}
}

// feedParityDelete replays feed_delete(...) for a tombstoned community post.
func feedParityDelete(db *gorm.DB, feedType string, postID int64) {
	if postID > math.MaxInt32 {
		return
	}
	if err := db.Exec("SELECT feed_delete(?, ?)", feedType, postID).Error; err != nil {
		slog.Warn("feed_activity parity delete failed (best-effort)", "post_id", postID, "type", feedType, "error", err)
	}
}

// An IMPORTED comment's feed row is keyed by its OLD comment id (written by the
// legacy trigger before the cutover), and the frozen old table's DELETE trigger
// can never fire again — so tombstoning an imported comment must also clear that
// legacy-keyed row, resolved through the import map ledger. 0 rows resolved =
// the post is post-cutover native, nothing to clear.

func feedParityDeleteLegacyGalgame(db *gorm.DB, postID int64) {
	feedParityDeleteLegacy(db, feedTypeGalgameComment,
		"SELECT old_comment_id FROM galgame_comment_community_map WHERE post_id = ?", postID)
}

func feedParityDeleteLegacyResource(db *gorm.DB, src CommentSource, postID int64) {
	feedParityDeleteLegacy(db, src.feedType,
		"SELECT old_id FROM resource_comment_community_map WHERE source = ? AND post_id = ?", src.key, postID)
}

func feedParityDeleteLegacy(db *gorm.DB, feedType, lookup string, args ...any) {
	var oldID int
	if err := db.Raw(lookup, args...).Scan(&oldID).Error; err != nil || oldID == 0 {
		return
	}
	if err := db.Exec("SELECT feed_delete(?, ?)", feedType, oldID).Error; err != nil {
		slog.Warn("feed_activity legacy-row delete failed (best-effort)", "old_id", oldID, "type", feedType, "error", err)
	}
}
