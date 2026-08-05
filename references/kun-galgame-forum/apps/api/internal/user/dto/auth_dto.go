package dto

import "time"

// ──────────────────────────────────────────
// Auth
// ──────────────────────────────────────────

type OAuthCallbackRequest struct {
	Code         string `json:"code" validate:"required,max=2048"`
	CodeVerifier string `json:"code_verifier" validate:"required,max=256"`
}

type SessionResponse struct {
	Token string       `json:"-"`
	User  *UserProfile `json:"user"`
}

// UserProfile is the shape returned by /auth/oauth/callback and /auth/me.
// Email is OAuth-owned; the frontend fetches it via OAuth /oauth/userinfo
// when needed. Identity here is sourced from OAuth via pkg/userclient.
type UserProfile struct {
	ID int `json:"id"`
	// Sub is the OAuth user UUID — the stable account identity used as login_hint
	// when switching accounts (the local integer id is not what OAuth keys on).
	Sub    string `json:"sub"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	// Roles is the raw OAuth role list — drives the account-switcher's admin
	// badge and the FE-derived creator capability.
	Roles       []string `json:"roles"`
	Moemoepoint int      `json:"moemoepoint"`
	Bio         string   `json:"bio"`
}

// ──────────────────────────────────────────
// User profile detail (GET /api/user/:userID)
// ──────────────────────────────────────────

type UserProfileDetail struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	// Roles is the raw OAuth role list; the FE derives both the role badge and
	// the creator capability from it.
	Roles       []string  `json:"roles"`
	Status      int       `json:"status"`
	Moemoepoint int       `json:"moemoepoint"`
	Bio         string    `json:"bio"`
	CreatedAt   time.Time `json:"created"`

	// Created counts
	Topic                  int64 `json:"topic"`
	TopicPoll              int64 `json:"topic_poll"`
	ReplyCreated           int64 `json:"reply_created"`
	CommentCreated         int64 `json:"comment_created"`
	Galgame                int64 `json:"galgame"`
	ContributeGalgame      int64 `json:"contribute_galgame"`
	GalgameComment         int64 `json:"galgame_comment"`
	GalgameRating          int64 `json:"galgame_rating"`
	GalgameResource        int64 `json:"galgame_resource"`
	GalgameToolset         int64 `json:"galgame_toolset"`
	GalgameToolsetResource int64 `json:"galgame_toolset_resource"`

	// Received interaction counts
	Upvote  int64 `json:"upvote"`
	Like    int64 `json:"like"`
	Dislike int64 `json:"dislike"`

	// Daily counts
	DailyTopicCount   int64 `json:"daily_topic_count"`
	DailyGalgameCount int64 `json:"daily_galgame_count"`
}

// ──────────────────────────────────────────
// User mutations
// ──────────────────────────────────────────

type UpdateBioRequest struct {
	Bio string `json:"bio" validate:"max=107"`
}

type UpdateUsernameRequest struct {
	Username string `json:"username" validate:"required,min=1,max=17"`
}

// ──────────────────────────────────────────
// User queries
// ──────────────────────────────────────────

type UserStatusResponse struct {
	Moemoepoints            int   `json:"moemoepoints"`
	IsCheckIn               bool  `json:"is_check_in"`
	HasNewMessage           bool  `json:"has_new_message"`
	DailyToolsetUploadBytes int64 `json:"daily_toolset_upload_bytes"`
	// IsCreator drives the avatar-menu "创作者申请" entry (hidden once held).
	// Derived from the live OAuth role (cached ~10min), same source as the
	// profile badge.
	IsCreator bool `json:"is_creator"`
}

type UserGalgamesRequest struct {
	Type  string `query:"type" validate:"required"`
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
	// ShowNoResource: false (default) hides galgames with no download resource
	// (the global "显示没有下载资源的 Galgame" toggle); true includes them.
	ShowNoResource bool `query:"show_no_resource"`
}

type UserTopicsRequest struct {
	Type  string `query:"type" validate:"required"`
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
}

type UserRepliesRequest struct {
	Type  string `query:"type" validate:"required"`
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
}

type UserCommentsRequest struct {
	Type  string `query:"type" validate:"required"`
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
}

type UserResourcesRequest struct {
	Type  string `query:"type" validate:"required"`
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=50"`
}

type UserRatingsRequest struct {
	Page  int `query:"page" validate:"min=1"`
	Limit int `query:"limit" validate:"min=1,max=50"`
}

type GalgameCard struct {
	ID           int       `json:"id"`
	VndbID       string    `json:"vndb_id"`
	NameEnUS     string    `json:"name_en_us"`
	NameJaJP     string    `json:"name_ja_jp"`
	NameZhCN     string    `json:"name_zh_cn"`
	NameZhTW     string    `json:"name_zh_tw"`
	Banner       string    `json:"banner"`
	ContentLimit string    `json:"content_limit"`
	CreatedAt    time.Time `json:"created"`
}

type UserTopic struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	// gorm:"column:created" is REQUIRED: FindUserTopics selects `topic.created`,
	// but GORM's default column for a `CreatedAt` field is `created_at`, so
	// without this the column never binds and the API serialises the zero value
	// "0001-01-01T00:00:00Z" (which the UI then renders as a 0001 date).
	CreatedAt time.Time `gorm:"column:created" json:"created"`
}

// ──────────────────────────────────────────
// Admin
// ──────────────────────────────────────────

type BanUserRequest struct {
	Status int `json:"status" validate:"oneof=0 1"`
}
