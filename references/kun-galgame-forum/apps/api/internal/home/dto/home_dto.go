package dto

import "time"

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type LocaleName struct {
	EnUS string `json:"en-us"`
	JaJP string `json:"ja-jp"`
	ZhCN string `json:"zh-cn"`
	ZhTW string `json:"zh-tw"`
}

type UserBrief struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type HomeGalgame struct {
	ID                 int        `json:"id"`
	Name               LocaleName `json:"name"`
	Banner             string     `json:"banner"`
	User               UserBrief  `json:"user"`
	ContentLimit       string     `json:"content_limit"`
	View               int        `json:"view"`
	LikeCount          int        `json:"like_count"`
	ResourceUpdateTime string     `json:"resource_update_time"`
	Platform           []string   `json:"platform"`
	Language           []string   `json:"language"`
	// U2: derived banner. effective_banner_url is injected by
	// client.rewriteBanners on every galgame response — dropping it here
	// forces the FE card to fall back to the legacy `banner` field,
	// which is empty for newly-uploaded (covers-only) galgames.
	EffectiveBannerHash string `json:"effective_banner_hash,omitempty"`
	EffectiveBannerURL  string `json:"effective_banner_url,omitempty"`
}

type HomeTopic struct {
	ID               int        `json:"id"`
	Title            string     `json:"title"`
	View             int        `json:"view"`
	LikeCount        int        `json:"like_count"`
	ReplyCount       int        `json:"reply_count"`
	CommentCount     int        `json:"comment_count"`
	HasBestAnswer    bool       `json:"has_best_answer"`
	IsPollTopic      bool       `json:"is_poll_topic"`
	IsNSFWTopic      bool       `json:"is_nsfw_topic"`
	Section          []string   `json:"section"`
	User             UserBrief  `json:"user"`
	Status           int        `json:"status"`
	UpvoteTime       *time.Time `json:"upvote_time"`
	StatusUpdateTime time.Time  `json:"status_update_time"`
}

type HomeResponse struct {
	Galgames []HomeGalgame `json:"galgames"`
	Topics   []HomeTopic   `json:"topics"`
}
