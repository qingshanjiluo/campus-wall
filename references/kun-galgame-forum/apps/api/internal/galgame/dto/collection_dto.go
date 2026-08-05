package dto

import "time"

// ──────────────────────────────────────────
// Galgame collection (收藏夹) request bodies
// ──────────────────────────────────────────

// CreateCollectionRequest — POST /galgame/collection.
type CreateCollectionRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=60"`
	Description string `json:"description" validate:"max=500"`
	Visibility  string `json:"visibility" validate:"required,oneof=public private restricted"`
	// ViewerIDs is honored only when visibility == "restricted".
	ViewerIDs []int `json:"viewer_ids" validate:"omitempty,max=100,dive,gt=0"`
}

// UpdateCollectionRequest — PATCH /galgame/collection/:cid. All fields optional
// (partial update); pointer fields distinguish "absent" from "cleared".
type UpdateCollectionRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=60"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	Visibility  *string `json:"visibility" validate:"omitempty,oneof=public private restricted"`
	ViewerIDs   *[]int  `json:"viewer_ids" validate:"omitempty,max=100,dive,gt=0"`
}

// SetCollectionMembershipRequest — PUT /galgame/:gid/collections. The FULL set
// of the user's collections that should contain this galgame; the service diffs
// against the current membership and adds/removes accordingly.
type SetCollectionMembershipRequest struct {
	CollectionIDs []int `json:"collection_ids" validate:"omitempty,max=50,dive,gt=0"`
}

// ──────────────────────────────────────────
// Response shapes
// ──────────────────────────────────────────

// CollectionSummary is one card in a user's collection grid (GET /user/:id/collections).
type CollectionSummary struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Visibility    string    `json:"visibility"`
	IsDefault     bool      `json:"is_default"`
	ItemCount     int       `json:"item_count"`
	PreviewCovers []string  `json:"preview_covers"`
	Created       time.Time `json:"created"`
	Updated       time.Time `json:"updated"`
}

// CollectionDetail is GET /galgame/collection/:cid — metadata + one page of games.
type CollectionDetail struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	IsDefault   bool      `json:"is_default"`
	ItemCount   int       `json:"item_count"`
	IsOwner     bool      `json:"is_owner"`
	Owner       UserBrief `json:"owner"`
	// Viewers is populated only for the owner of a restricted collection, so the
	// edit form can prefill the allow-list.
	Viewers  []UserBrief       `json:"viewers"`
	Galgames []GalgameListCard `json:"galgames"`
	Total    int64             `json:"total"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
}

// MyCollectionForGalgame is one row in the collection-picker modal
// (GET /galgame/:gid/collections/mine): the user's collection + whether this
// galgame is already in it.
type MyCollectionForGalgame struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	IsDefault  bool   `json:"is_default"`
	ItemCount  int    `json:"item_count"`
	Contains   bool   `json:"contains"`
}
