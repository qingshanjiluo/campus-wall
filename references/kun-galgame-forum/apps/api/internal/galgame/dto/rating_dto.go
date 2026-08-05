package dto

import "encoding/json"

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type RatingListRequest struct {
	Page         int    `query:"page" validate:"min=1"`
	Limit        int    `query:"limit" validate:"min=1,max=50"`
	SortField    string `query:"sort_field"`
	SortOrder    string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
	SpoilerLevel string `query:"spoiler_level"`
	PlayStatus   string `query:"play_status"`
	GalgameType  string `query:"galgame_type"`
}

// CreateRatingRequest is the body of POST /galgame-rating.
// short_summary length drives the moemoepoint reward tier.
type CreateRatingRequest struct {
	GalgameID    int      `json:"galgame_id" validate:"required,min=1"`
	Recommend    string   `json:"recommend" validate:"required"`
	Overall      int      `json:"overall" validate:"required,min=1,max=10"`
	GalgameType  []string `json:"galgame_type" validate:"required,min=1"`
	PlayStatus   string   `json:"play_status" validate:"required"`
	ShortSummary string   `json:"short_summary" validate:"max=1314"`
	SpoilerLevel string   `json:"spoiler_level"`
	Art          int      `json:"art" validate:"min=0,max=10"`
	Story        int      `json:"story" validate:"min=0,max=10"`
	Music        int      `json:"music" validate:"min=0,max=10"`
	Character    int      `json:"character" validate:"min=0,max=10"`
	Route        int      `json:"route" validate:"min=0,max=10"`
	System       int      `json:"system" validate:"min=0,max=10"`
	Voice        int      `json:"voice" validate:"min=0,max=10"`
	ReplayValue  int      `json:"replay_value" validate:"min=0,max=10"`
}

// UpdateRatingRequest is the body of PUT /galgame-rating/:id.
type UpdateRatingRequest struct {
	GalgameRatingID int      `json:"galgame_rating_id" validate:"required,min=1"`
	Recommend       string   `json:"recommend" validate:"required"`
	Overall         int      `json:"overall" validate:"required,min=1,max=10"`
	GalgameType     []string `json:"galgame_type" validate:"required,min=1"`
	PlayStatus      string   `json:"play_status" validate:"required"`
	ShortSummary    string   `json:"short_summary" validate:"max=1314"`
	SpoilerLevel    string   `json:"spoiler_level" validate:"required"`
	Art             int      `json:"art" validate:"min=0,max=10"`
	Story           int      `json:"story" validate:"min=0,max=10"`
	Music           int      `json:"music" validate:"min=0,max=10"`
	Character       int      `json:"character" validate:"min=0,max=10"`
	Route           int      `json:"route" validate:"min=0,max=10"`
	System          int      `json:"system" validate:"min=0,max=10"`
	Voice           int      `json:"voice" validate:"min=0,max=10"`
	ReplayValue     int      `json:"replay_value" validate:"min=0,max=10"`
}

// DeleteRatingRequest is the query for DELETE /galgame-rating/:id.
type DeleteRatingRequest struct {
	GalgameRatingID int `query:"galgame_rating_id" validate:"required,min=1"`
}

// ToggleRatingLikeRequest is the body of PUT /galgame-rating/:id/like.
type ToggleRatingLikeRequest struct {
	GalgameRatingID int `json:"galgame_rating_id" validate:"required,min=1"`
}

// The rating comment CRUD (POST/PUT/DELETE /galgame-rating/:id/comment) was
// retired in charter step 06a — comments moved to the community primitive and
// the galgame_rating_comment table was dropped (migration 060).

// CreatedRating is the response shape for POST /galgame-rating — matches
// GalgameRatingCardOnGalgamePage in the frontend types.
type CreatedRating struct {
	ID           int                `json:"id"`
	User         UserBrief          `json:"user"`
	Recommend    string             `json:"recommend"`
	Overall      int                `json:"overall"`
	View         int                `json:"view"`
	GalgameType  json.RawMessage    `json:"galgame_type"`
	PlayStatus   string             `json:"play_status"`
	ShortSummary string             `json:"short_summary"`
	SpoilerLevel string             `json:"spoiler_level"`
	RatingScores                    // embedded scores
	LikeCount    int                `json:"like_count"`
	IsLiked      bool               `json:"is_liked"`
	Created      string             `json:"created"`
	Updated      string             `json:"updated"`
	Galgame      RatingGalgameBrief `json:"galgame"`
}

// ──────────────────────────────────────────
// Shared scores embed
// ──────────────────────────────────────────

// RatingScores holds the per-axis rating scores.
// Spread into rating card + detail to match the frontend shape.
type RatingScores struct {
	Art         int `json:"art"`
	Story       int `json:"story"`
	Music       int `json:"music"`
	Character   int `json:"character"`
	Route       int `json:"route"`
	System      int `json:"system"`
	Voice       int `json:"voice"`
	ReplayValue int `json:"replay_value"`
}

// ──────────────────────────────────────────
// Responses — list
// ──────────────────────────────────────────

// RatingGalgameBrief is the lightweight galgame info in rating lists.
type RatingGalgameBrief struct {
	ID           int         `json:"id"`
	ContentLimit string      `json:"content_limit"`
	Name         KunLanguage `json:"name"`
}

// RatingCard is a single entry in the rating list response.
type RatingCard struct {
	ID           int                `json:"id"`
	User         UserBrief          `json:"user"`
	Recommend    string             `json:"recommend"`
	Overall      int                `json:"overall"`
	View         int                `json:"view"`
	GalgameType  json.RawMessage    `json:"galgame_type"`
	PlayStatus   string             `json:"play_status"`
	ShortSummary string             `json:"short_summary"`
	SpoilerLevel string             `json:"spoiler_level"`
	RatingScores                    // embedded fields art/story/music/...
	LikeCount    int                `json:"like_count"`
	Created      string             `json:"created"`
	Updated      string             `json:"updated"`
	Galgame      RatingGalgameBrief `json:"galgame"`
}

type RatingListPage struct {
	RatingData []RatingCard `json:"rating_data"`
	Total      int64        `json:"total"`
}

// ──────────────────────────────────────────
// Responses — detail
// ──────────────────────────────────────────

// RatingOfficial is a single official/company entry shown in rating detail.
type RatingOfficial struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Link         string   `json:"link"`
	Category     string   `json:"category"`
	Lang         string   `json:"lang"`
	Alias        []string `json:"alias"`
	GalgameCount int      `json:"galgame_count"`
}

// RatingGalgameDetail is the full galgame panel on the rating detail page.
type RatingGalgameDetail struct {
	ID           int    `json:"id"`
	ContentLimit string `json:"content_limit"`
	Banner       string `json:"banner"`
	// U2 banner pair — FE getEffectiveBanner reads these first and only
	// falls back to legacy `banner` when both are empty. New
	// covers-only galgames (post galgame PR5) have empty `banner`, so
	// omitting these here renders an empty hero on the rating page.
	EffectiveBannerHash      string           `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL       string           `json:"effective_banner_url,omitempty"`
	EffectiveBannerWidth     int              `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int              `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string           `json:"effective_banner_thumbhash,omitempty"`
	AgeLimit                 string           `json:"age_limit"`
	OriginalLanguage         string           `json:"original_language"`
	Rating                   int64            `json:"rating"`
	RatingCount              int64            `json:"rating_count"`
	Official                 []RatingOfficial `json:"official"`
	Name                     KunLanguage      `json:"name"`
}

// RatingDetail is the full response for GET /galgame-rating/:id.
type RatingDetail struct {
	ID           int                 `json:"id"`
	User         UserBrief           `json:"user"`
	Recommend    string              `json:"recommend"`
	Overall      int                 `json:"overall"`
	View         int                 `json:"view"`
	GalgameType  json.RawMessage     `json:"galgame_type"`
	PlayStatus   string              `json:"play_status"`
	ShortSummary string              `json:"short_summary"`
	SpoilerLevel string              `json:"spoiler_level"`
	RatingScores                     // embedded art/story/music/...
	LikeCount    int                 `json:"like_count"`
	IsLiked      bool                `json:"is_liked"`
	LikedUsers   []UserBrief         `json:"liked_users"`
	Created      string              `json:"created"`
	Updated      string              `json:"updated"`
	Galgame      RatingGalgameDetail `json:"galgame"`
}
