package dto

// UserRatingGalgame is the embedded galgame metadata on a rating.
type UserRatingGalgame struct {
	ID           int         `json:"id"`
	Name         KunLanguage `json:"name"`
	ContentLimit string      `json:"content_limit"`
}

// UserRatingItem mirrors the legacy rating-list response shape. Field names
// are intentionally a mix of camelCase and snake_case to match the existing
// frontend contract — do NOT rename without also updating the client.
//
// Shape parity with shared/types/galgame-rating.ts GalgameRatingCard is
// load-bearing: components/user/Rating.vue passes this slice straight
// to <GalgameRatingCard /> which extends GalgameRatingCard, so any
// field missing here becomes `undefined` on the FE.
type UserRatingItem struct {
	ID          int       `json:"id"`
	User        UserBrief `json:"user"`
	Recommend   string    `json:"recommend"`
	Overall     int       `json:"overall"`
	View        int       `json:"view"`
	GalgameType []string  `json:"galgame_type"`
	PlayStatus  string    `json:"play_status"`
	// Author writeup; mirrors RatingCard.short_summary. Without it the
	// user rating cards never carry the preview text (K-PR6 added the
	// field to the FE type but the user-scoped DTO was missed).
	ShortSummary string            `json:"short_summary"`
	Art          int               `json:"art"`
	Story        int               `json:"story"`
	Music        int               `json:"music"`
	Character    int               `json:"character"`
	Route        int               `json:"route"`
	System       int               `json:"system"`
	Voice        int               `json:"voice"`
	ReplayValue  int               `json:"replay_value"`
	SpoilerLevel string            `json:"spoiler_level"`
	LikeCount    int               `json:"like_count"`
	Created      string            `json:"created"`
	Updated      string            `json:"updated"`
	Galgame      UserRatingGalgame `json:"galgame"`
}

// UserRatingsResponse is the payload for GET /api/user/:userID/ratings.
type UserRatingsResponse struct {
	RatingData []UserRatingItem `json:"rating_data"`
	Total      int64            `json:"total"`
}
