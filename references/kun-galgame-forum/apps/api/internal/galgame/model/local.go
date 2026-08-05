package model

import "time"

// GalgameLocal represents the stripped-down local galgame row.
// After galgame migration, only interaction counts + view remain locally.
//
// CreatedAt / UpdatedAt are needed so the lazy-create stub
// (`Create(&GalgameLocal{ID: ...}).OnConflict(DoNothing)`) emits valid
// timestamps for a possibly-new row. The `galgame.updated` column is
// NOT NULL with no DB-level default — left over from the Prisma
// `@updatedAt` directive which Prisma fills application-side. PG
// evaluates NOT NULL BEFORE resolving ON CONFLICT, so omitting the
// column trips a constraint violation even when the row already
// exists. GORM auto-populates these by field-name convention.
type GalgameLocal struct {
	ID               int `gorm:"primaryKey" json:"id"`
	View             int `gorm:"default:0" json:"view"`
	LikeCount        int `gorm:"column:like_count;default:0" json:"like_count"`
	FavoriteCount    int `gorm:"column:favorite_count;default:0" json:"favorite_count"`
	ResourceCount    int `gorm:"column:resource_count;default:0" json:"resource_count"`
	CommentCount     int `gorm:"column:comment_count;default:0" json:"comment_count"`
	ContributorCount int `gorm:"column:contributor_count;default:0" json:"contributor_count"`
	RatingCount      int `gorm:"column:rating_count;default:0" json:"rating_count"`
	// Mirror of galgame's release_date (migration 013), so the local browse
	// list can filter/sort by release year/month — kungal's /galgame
	// doesn't proxy galgame's list. Nullable: NULL = unknown / not yet
	// backfilled. Populated by cmd/backfill-release-date (idempotent).
	// The lazy-create stub leaves it nil → NULL until a backfill run.
	ReleaseDate *time.Time `gorm:"column:release_date" json:"release_date"`
	CreatedAt   time.Time  `gorm:"column:created" json:"created"`
	UpdatedAt   time.Time  `gorm:"column:updated" json:"updated"`
	// ResourceUpdateTime is the dedicated content-update sort key (migration
	// 018), separate from the generic audit `updated`. autoCreateTime means GORM
	// sets it once on the lazy-create stub and NEVER bumps it on a plain
	// Save/Update — only the explicit Touch / TouchGalgameUpdated content paths
	// move it, so engagement (like / favorite / comment / view) can't reorder
	// the list.
	ResourceUpdateTime time.Time `gorm:"column:resource_update_time;autoCreateTime" json:"resource_update_time"`
	// ResourcePublishBanned is a moderator kill-switch (migration 061): while
	// true, publishing / editing download resources under this galgame is
	// forbidden (copyright-holder notice or other third-party takedown).
	// Enforced in resource_service.go; toggled via the moderator ban endpoint.
	ResourcePublishBanned bool `gorm:"column:resource_publish_banned;default:false" json:"resource_publish_banned"`
	// CreatorUserID is a WRITE-ONCE attribution snapshot. The catalog face
	// carries no product's submitter by design (doc 106 R2), so the author
	// chip kungal renders on every galgame card is served locally: migrated
	// rows hold the frozen wiki-era submitter (migration 066,
	// cmd/backfill-galgame-creator), registry-born rows the claimant recorded
	// by the claim-event cron when the entry first went live
	// (SetCreatorIfUnset — never overwritten by later republishes).
	// nil = unknown, which renders as no author chip rather than as user 0.
	CreatorUserID *int `gorm:"column:creator_user_id" json:"creator_user_id"`
}

func (GalgameLocal) TableName() string { return "galgame" }

// ──────────────────────────────────────────
// Interactions (local to each site)
// ──────────────────────────────────────────

type GalgameLike struct {
	ID        int `gorm:"primaryKey;autoIncrement" json:"id"`
	GalgameID int `gorm:"column:galgame_id;not null;uniqueIndex:idx_galgame_like" json:"galgame_id"`
	UserID    int `gorm:"column:user_id;not null;uniqueIndex:idx_galgame_like" json:"user_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameLike) TableName() string { return "galgame_like" }

type GalgameFavorite struct {
	ID        int `gorm:"primaryKey;autoIncrement" json:"id"`
	GalgameID int `gorm:"column:galgame_id;not null;uniqueIndex:idx_galgame_favorite" json:"galgame_id"`
	UserID    int `gorm:"column:user_id;not null;uniqueIndex:idx_galgame_favorite" json:"user_id"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameFavorite) TableName() string { return "galgame_favorite" }

// The legacy local comment tables (galgame_comment + galgame_comment_like) were
// retired in charter step 06a — galgame comments now live on the infra community
// primitive (see community_comment_service.go). The frozen tables are dropped by
// migration 060; the community post-like table lives in model/community_post.go.
