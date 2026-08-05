package dto

// GetTagsRequest is the query for GET /doc/tag.
type GetTagsRequest struct {
	Page    int    `query:"page" validate:"min=1"`
	Limit   int    `query:"limit" validate:"min=1,max=100"`
	Keyword string `query:"keyword"`
}

// DeleteTagRequest is the query for DELETE /doc/tag.
type DeleteTagRequest struct {
	TagID int `query:"tag_id" validate:"required,min=1"`
}

// CreateTagRequest is the body for POST /doc/tag. Dedicated DTO instead
// of binding directly to model.DocTag — same rationale as
// CreateCategoryRequest: explicit validate tags reject empty / too-long
// inputs at the boundary.
type CreateTagRequest struct {
	Slug        string `json:"slug" validate:"required,min=1,max=100"`
	Title       string `json:"title" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}

// UpdateTagRequest is the body for PUT /doc/tag.
type UpdateTagRequest struct {
	TagID       int    `json:"tag_id" validate:"required,min=1"`
	Slug        string `json:"slug" validate:"required,min=1,max=100"`
	Title       string `json:"title" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
}
