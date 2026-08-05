package repository

import (
	"encoding/json"
	"strings"
	"time"

	"kun-galgame-api/internal/galgame/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// pgTextArrayLiteral renders a []string as a Postgres text[] literal,
// e.g. ["baidu", "quark"] → `{"baidu","quark"}`. Empty/nil input yields
// `{}` (an empty text[]).
//
// GORM has no native binding for Postgres array types — passing
// `[]string{...}` as a positional parameter expands to `('baidu')`,
// which the server parses as a parenthesised scalar (not an array)
// and rejects with `malformed array literal`. Producing the literal
// ourselves and casting via `?::text[]` is the dep-free workaround
// (avoids pulling in `github.com/lib/pq` just for `pq.StringArray`).
//
// Each element is double-quoted, with `\` and `"` escaped — this keeps
// the format safe even if a provider key ever contains `,`, `{`, `}`,
// or whitespace. (Today our keys are simple ASCII identifiers from
// pkg/utils/provider.go, but we don't want to invent landmines.)
func pgTextArrayLiteral(items []string) string {
	if len(items) == 0 {
		return "{}"
	}
	parts := make([]string, len(items))
	for i, v := range items {
		escaped := strings.ReplaceAll(v, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		parts[i] = `"` + escaped + `"`
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// onConflictNothing is shared for batch inserts that should tolerate uniqueness
// collisions (e.g. re-inserting resource links after an edit).
var onConflictNothing = clause.OnConflict{DoNothing: true}

type ResourceRepository struct {
	db *gorm.DB
}

func NewResourceRepository(db *gorm.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

// DB exposes the underlying connection for transaction orchestration in services.
func (r *ResourceRepository) DB() *gorm.DB { return r.db }

// ──────────────────────────────────────────
// Reads
// ──────────────────────────────────────────

// CountAll returns the total number of resources.
func (r *ResourceRepository) CountAll() int64 {
	var total int64
	r.db.Table("galgame_resource").Count(&total)
	return total
}

// ListPaginated returns resources ordered by created DESC.
func (r *ResourceRepository) ListPaginated(page, limit int) []model.GalgameResourceRow {
	offset := (page - 1) * limit
	var rows []model.GalgameResourceRow
	r.db.Table("galgame_resource").
		Order("created DESC").
		Offset(offset).Limit(limit).
		Scan(&rows)
	return rows
}

// FindByID returns a single resource row. Returns false if not found.
func (r *ResourceRepository) FindByID(id int) (model.GalgameResourceRow, bool) {
	var row model.GalgameResourceRow
	if err := r.db.Table("galgame_resource").Where("id = ?", id).Scan(&row).Error; err != nil || row.ID == 0 {
		return row, false
	}
	return row, true
}

// IsResourcePublishBanned reports whether a moderator has forbidden publishing
// download resources under this galgame (migration 061). No local row (a
// never-ingested galgame game) ⇒ not banned.
func (r *ResourceRepository) IsResourcePublishBanned(galgameID int) bool {
	var banned []bool
	r.db.Table("galgame").Where("id = ?", galgameID).Pluck("resource_publish_banned", &banned)
	return len(banned) > 0 && banned[0]
}

// SetResourcePublishBanned upserts the local galgame row and sets the ban flag,
// so a moderator can pre-ban a galgame game the forum has never ingested.
func (r *ResourceRepository) SetResourcePublishBanned(galgameID int, banned bool) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{"resource_publish_banned": banned}),
	}).Create(&model.GalgameLocal{ID: galgameID, ResourcePublishBanned: banned}).Error
}

// FindByGalgameID returns all resources for a galgame ordered so that valid
// resources (status 0) come first in RANDOM order and expired ones (status 1)
// always sink to the bottom. The detail page groups this list into provider
// buckets while preserving order, so each bucket shows shuffled-valid-then-
// expired-last.
func (r *ResourceRepository) FindByGalgameID(galgameID int) []model.GalgameResourceRow {
	var rows []model.GalgameResourceRow
	r.db.Table("galgame_resource").
		Where("galgame_id = ?", galgameID).
		Order("status ASC, RANDOM()").
		Scan(&rows)
	return rows
}

// FindRecommendations returns other resources in the same galgame, sorted by
// like_count DESC, limited to `limit`.
func (r *ResourceRepository) FindRecommendations(galgameID, excludeID, limit int) []model.GalgameResourceRow {
	var rows []model.GalgameResourceRow
	r.db.Table("galgame_resource").
		Where("galgame_id = ? AND id != ?", galgameID, excludeID).
		Order("like_count DESC").
		Limit(limit).
		Scan(&rows)
	return rows
}

// FindLinks returns all download URLs for a resource.
func (r *ResourceRepository) FindLinks(resourceID int) []string {
	type linkRow struct {
		URL string `gorm:"column:url"`
	}
	var links []linkRow
	r.db.Table("galgame_resource_link").
		Where("galgame_resource_id = ?", resourceID).
		Scan(&links)
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.URL
	}
	return out
}

// AggregateByGalgame returns DISTINCT (platform, language, type) tuples.
func (r *ResourceRepository) AggregateByGalgame(galgameID int) []model.ResourceAggregate {
	var aggs []model.ResourceAggregate
	r.db.Table("galgame_resource").
		Select("DISTINCT platform, language, type").
		Where("galgame_id = ?", galgameID).
		Scan(&aggs)
	return aggs
}

// IsLikedBy checks whether a user has liked a given resource.
func (r *ResourceRepository) IsLikedBy(resourceID, userID int) bool {
	if userID <= 0 {
		return false
	}
	var cnt int64
	r.db.Table("galgame_resource_like").
		Where("galgame_resource_id = ? AND user_id = ?", resourceID, userID).
		Count(&cnt)
	return cnt > 0
}

// FindLikedSet returns the subset of resourceIDs that `userID` has
// liked. Empty result for anonymous (userID=0) or empty input — saves
// the N+1 single-row Count calls that GetGalgameResources / list paths
// would otherwise need to compute the per-row `isLiked` flag.
func (r *ResourceRepository) FindLikedSet(userID int, resourceIDs []int) map[int]bool {
	out := map[int]bool{}
	if userID <= 0 || len(resourceIDs) == 0 {
		return out
	}
	var rows []struct {
		ResourceID int `gorm:"column:galgame_resource_id"`
	}
	r.db.Table("galgame_resource_like").
		Where("user_id = ? AND galgame_resource_id IN ?", userID, resourceIDs).
		Select("galgame_resource_id").
		Scan(&rows)
	for _, row := range rows {
		out[row.ResourceID] = true
	}
	return out
}

// FindGalgameView returns the local galgame.view counter.
func (r *ResourceRepository) FindGalgameView(galgameID int) int {
	var view int
	r.db.Table("galgame").Select("view").Where("id = ?", galgameID).Scan(&view)
	return view
}

// ──────────────────────────────────────────
// Writes
// ──────────────────────────────────────────

// IncrementView atomically bumps the view count.
func (r *ResourceRepository) IncrementView(resourceID int) {
	r.db.Exec("UPDATE galgame_resource SET view = view + 1 WHERE id = ?", resourceID)
}

// IncrementDownload atomically bumps the download count.
func (r *ResourceRepository) IncrementDownload(resourceID int) {
	r.db.Exec("UPDATE galgame_resource SET download = download + 1 WHERE id = ?", resourceID)
}

// Create inserts a new galgame_resource row within the given tx.
func (r *ResourceRepository) Create(tx *gorm.DB, res *model.GalgameResource) error {
	return tx.Create(res).Error
}

// ReplaceProviders overwrites the text[] provider column for a resource.
// GORM has no native array type, so this uses raw SQL with a manually-
// formatted Postgres array literal (see pgTextArrayLiteral above for
// why a plain `?` placeholder won't work).
func (r *ResourceRepository) ReplaceProviders(tx *gorm.DB, resourceID int, providers []string) error {
	return tx.Exec(
		"UPDATE galgame_resource SET provider = ?::text[] WHERE id = ?",
		pgTextArrayLiteral(providers), resourceID,
	).Error
}

// ReplaceProviderNames overwrites the jsonb provider_name column with the
// given display labels (e.g. ["百度网盘", "OneDrive"]). Stored as JSON text;
// the read side either passes through the raw bytes or unmarshals to []string.
func (r *ResourceRepository) ReplaceProviderNames(tx *gorm.DB, resourceID int, names []string) error {
	if names == nil {
		names = []string{}
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return tx.Exec(
		"UPDATE galgame_resource SET provider_name = ?::jsonb WHERE id = ?",
		string(encoded), resourceID,
	).Error
}

// CreateLinks inserts resource_link rows, skipping (resource_id, url) duplicates.
func (r *ResourceRepository) CreateLinks(tx *gorm.DB, resourceID int, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	links := make([]model.GalgameResourceLink, len(urls))
	for i, u := range urls {
		links[i] = model.GalgameResourceLink{GalgameResourceID: resourceID, URL: u}
	}
	return tx.Clauses(onConflictNothing).Create(&links).Error
}

// DeleteLinks removes all links for a resource.
func (r *ResourceRepository) DeleteLinks(tx *gorm.DB, resourceID int) error {
	return tx.Where("galgame_resource_id = ?", resourceID).
		Delete(&model.GalgameResourceLink{}).Error
}

// UpdateFields patches arbitrary columns on a resource row (used for edit endpoint).
// Keys are column names (not struct field names).
func (r *ResourceRepository) UpdateFields(tx *gorm.DB, resourceID int, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return tx.Table("galgame_resource").Where("id = ?", resourceID).
		Updates(fields).Error
}

// UpdateStatus sets the status column (0 = valid, 1 = expired).
func (r *ResourceRepository) UpdateStatus(tx *gorm.DB, resourceID, status int) error {
	return tx.Table("galgame_resource").Where("id = ?", resourceID).
		Update("status", status).Error
}

// DeleteByID removes a resource by primary key. Cascades to links/likes via FK.
func (r *ResourceRepository) DeleteByID(tx *gorm.DB, resourceID int) error {
	return tx.Where("id = ?", resourceID).Delete(&model.GalgameResource{}).Error
}

// FindLike returns the like row for (resource, user), or ok=false.
func (r *ResourceRepository) FindLike(tx *gorm.DB, resourceID, userID int) (model.GalgameResourceLike, bool) {
	var like model.GalgameResourceLike
	err := tx.Where("galgame_resource_id = ? AND user_id = ?", resourceID, userID).
		First(&like).Error
	if err != nil {
		return like, false
	}
	return like, true
}

// CreateLike adds a like row for (resource, user).
func (r *ResourceRepository) CreateLike(tx *gorm.DB, resourceID, userID int) error {
	return tx.Create(&model.GalgameResourceLike{
		GalgameResourceID: resourceID, UserID: userID,
	}).Error
}

// DeleteLike removes a specific like row.
func (r *ResourceRepository) DeleteLike(tx *gorm.DB, like model.GalgameResourceLike) error {
	return tx.Delete(&like).Error
}

// AdjustLikeCount adjusts the resource.like_count counter by delta (+1/-1).
func (r *ResourceRepository) AdjustLikeCount(tx *gorm.DB, resourceID, delta int) error {
	return tx.Table("galgame_resource").Where("id = ?", resourceID).
		Update("like_count", gorm.Expr("like_count + ?", delta)).Error
}

// AdjustLocalResourceCount adjusts galgame.resource_count by delta. Uses
// .Table(...).Update(...) which BYPASSES GORM's autoUpdateTime — so adjusting
// the count alone does NOT bump `updated`. Bumping on publish/edit is done
// explicitly via TouchGalgameUpdated; delete intentionally only decrements.
func (r *ResourceRepository) AdjustLocalResourceCount(tx *gorm.DB, galgameID, delta int) error {
	return tx.Table("galgame").Where("id = ?", galgameID).
		Update("resource_count", gorm.Expr("resource_count + ?", delta)).Error
}

// TouchGalgameUpdated bumps galgame.resource_update_time to now to mark a
// resource content change (publish / edit). Separate from
// AdjustLocalResourceCount so a resource DELETE can decrement the count WITHOUT
// promoting the galgame in the "sort by update time" list.
func (r *ResourceRepository) TouchGalgameUpdated(tx *gorm.DB, galgameID int) error {
	return tx.Table("galgame").Where("id = ?", galgameID).
		UpdateColumn("resource_update_time", time.Now()).Error
}
