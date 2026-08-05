package dto

import (
	"time"
)

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// `Description` requires min=10 and `Icon` is validated as a URL —
// previously the FE schema enforced these but the BE DTO didn't, so the
// BE silently accepted empty descriptions and non-URL icons that the FE
// would have rejected. Tightened the BE to match what the product
// actually requires.
//
// `URL` is the site's BARE main domain (no scheme — e.g. `www.kungal.com`),
// which is how every row is stored and how the UI links to it
// (`https://${url}`). The `url` tag requires a scheme, so it rejected every
// existing entry on edit/create; `fqdn` validates a bare domain instead.
// `Icon`, by contrast, IS a full URL, so it keeps the `url` tag.
type CreateWebsiteRequest struct {
	Name          string   `json:"name" validate:"required,max=233"`
	URL           string   `json:"url" validate:"required,fqdn,max=500"`
	Description   string   `json:"description" validate:"required,min=10,max=1000"`
	Icon          string   `json:"icon" validate:"omitempty,url,max=500"`
	IconImageHash string   `json:"icon_image_hash" validate:"max=128"`
	CategoryID    int      `json:"category_id" validate:"required,min=1"`
	AgeLimit      string   `json:"age_limit" validate:"required,oneof=all r18"`
	Language      string   `json:"language" validate:"max=10"`
	TagIDs        []int    `json:"tag_ids"`
	Domain        []string `json:"domain" validate:"max=10,dive,max=100"`
	CreateTime    string   `json:"create_time" validate:"max=20"`
}

type UpdateWebsiteRequest struct {
	WebsiteID     int      `json:"website_id" validate:"required,min=1"`
	Name          string   `json:"name" validate:"required,max=233"`
	URL           string   `json:"url" validate:"required,fqdn,max=500"`
	Description   string   `json:"description" validate:"required,min=10,max=1000"`
	Icon          string   `json:"icon" validate:"omitempty,url,max=500"`
	IconImageHash string   `json:"icon_image_hash" validate:"max=128"`
	CategoryID    int      `json:"category_id" validate:"required,min=1"`
	AgeLimit      string   `json:"age_limit" validate:"required,oneof=all r18"`
	Language      string   `json:"language" validate:"max=10"`
	TagIDs        []int    `json:"tag_ids"`
	Domain        []string `json:"domain" validate:"max=10,dive,max=100"`
	CreateTime    string   `json:"create_time" validate:"max=20"`
}

type DeleteWebsiteRequest struct {
	WebsiteID int `query:"website_id" validate:"required,min=1"`
}

type ToggleInteractionRequest struct {
	WebsiteID int `json:"website_id" validate:"required,min=1"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

// WebsiteCard is the list-card shape returned by GetWebsites and nested in
// category/tag detail responses.
type WebsiteCard struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Domain        string `json:"domain"`
	AgeLimit      string `json:"age_limit"`
	Level         int    `json:"level"`
	Icon          string `json:"icon"`
	IconImageHash string `json:"icon_image_hash"`
	IconURL       string `json:"icon_url"`
	Price         int    `json:"price"`
	Category      string `json:"category"`
}

// WebsiteCategoryBrief is the category sub-object in the website detail response.
type WebsiteCategoryBrief struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// WebsiteTagBrief is the tag sub-object in the website detail response.
type WebsiteTagBrief struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Label       string `json:"label"`
	Level       int    `json:"level"`
}

// UserBriefCompact is the user projection used for comments in the website detail.
type UserBriefCompact struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// WebsiteDetailComment is a simple flat comment attached to the website detail.
type WebsiteDetailComment struct {
	ID      int              `json:"id"`
	Content string           `json:"content"`
	User    UserBriefCompact `json:"user"`
	Created string           `json:"created"`
	Updated string           `json:"updated"`
}

// WebsiteDetailResponse is the shape of GET /website/:domain.
// All field names and ordering are preserved verbatim from the original handler.
type WebsiteDetailResponse struct {
	ID            int                    `json:"id"`
	Name          string                 `json:"name"`
	URL           string                 `json:"url"`
	Description   string                 `json:"description"`
	Icon          string                 `json:"icon"`
	IconImageHash string                 `json:"icon_image_hash"`
	IconURL       string                 `json:"icon_url"`
	View          int                    `json:"view"`
	Language      string                 `json:"language"`
	AgeLimit      string                 `json:"age_limit"`
	Category      WebsiteCategoryBrief   `json:"category"`
	Tags          []WebsiteTagBrief      `json:"tags"`
	LikeCount     int                    `json:"like_count"`
	IsLiked       bool                   `json:"is_liked"`
	FavoriteCount int                    `json:"favorite_count"`
	IsFavorited   bool                   `json:"is_favorited"`
	Domain        any                    `json:"domain"`
	CreateTime    string                 `json:"create_time"`
	Comment       []WebsiteDetailComment `json:"comment"`
	Created       time.Time              `json:"created"`
	Updated       time.Time              `json:"updated"`
}
