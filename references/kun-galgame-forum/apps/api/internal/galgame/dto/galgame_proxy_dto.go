package dto

// ──────────────────────────────────────────
// Galgame Links (/galgame/:gid/link/all)
// ──────────────────────────────────────────

// GalgameLink is an external link attached to a galgame, with the creator user
// resolved from the local DB.
type GalgameLink struct {
	ID        int       `json:"id"`
	User      UserBrief `json:"user"`
	GalgameID int       `json:"galgame_id"`
	Name      string    `json:"name"`
	Link      string    `json:"link"`
}
