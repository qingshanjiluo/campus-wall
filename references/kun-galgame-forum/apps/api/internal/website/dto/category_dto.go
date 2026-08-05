package dto

import "time"

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// Caps mirror apps/web/app/validations/website.ts updateWebsiteCategorySchema.
// The BE was silently accepting unbounded name/label/description — added
// max constraints so direct-API callers can't slip 5000-char categories
// into the DB.
type UpdateWebsiteCategoryRequest struct {
	CategoryID  int    `json:"category_id" validate:"required,min=1"`
	Name        string `json:"name" validate:"required,min=1,max=30"`
	Label       string `json:"label" validate:"required,min=1,max=30"`
	Description string `json:"description" validate:"max=300"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

// WebsiteCategoryDetailResponse is the shape of GET /website-category/:name.
// Field names mirror the pre-refactor handler output exactly.
type WebsiteCategoryDetailResponse struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Label        string        `json:"label"`
	Description  string        `json:"description"`
	WebsiteCount int           `json:"website_count"`
	Websites     []WebsiteCard `json:"websites"`
	Created      time.Time     `json:"created"`
	Updated      time.Time     `json:"updated"`
}
