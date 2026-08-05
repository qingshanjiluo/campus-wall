package dto

// Categories are the 3 fixed display groups (mirror the frontend headers).
// oneof= in the validate tags below is the source of truth enforced per-request.

type CreateRequest struct {
	Category        string `json:"category" validate:"required,oneof=official galgame others"`
	Name            string `json:"name" validate:"required,max=100"`
	Link            string `json:"link" validate:"required,url,max=500"`
	Description     string `json:"description" validate:"max=500"`
	Banner          string `json:"banner" validate:"max=500"`
	BannerImageHash string `json:"banner_image_hash" validate:"max=128"`
	Status          string `json:"status" validate:"omitempty,oneof=normal essential down"`
}

type UpdateRequest struct {
	ID              int    `json:"id" validate:"required,min=1"`
	Category        string `json:"category" validate:"required,oneof=official galgame others"`
	Name            string `json:"name" validate:"required,max=100"`
	Link            string `json:"link" validate:"required,url,max=500"`
	Description     string `json:"description" validate:"max=500"`
	Banner          string `json:"banner" validate:"max=500"`
	BannerImageHash string `json:"banner_image_hash" validate:"max=128"`
	Status          string `json:"status" validate:"omitempty,oneof=normal essential down"`
}

type DeleteRequest struct {
	ID int `query:"id" validate:"required,min=1"`
}

// ReorderRequest carries one category's full new ordering (the dragged list of
// ids, top → bottom). sort_order is rewritten to each id's index.
type ReorderRequest struct {
	Category string `json:"category" validate:"required,oneof=official galgame others"`
	IDs      []int  `json:"ids" validate:"required,min=1,dive,min=1"`
}
