package dto

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// GetCategoriesRequest is the query for GET /doc/category.
type GetCategoriesRequest struct {
	Page    int    `query:"page" validate:"min=1"`
	Limit   int    `query:"limit" validate:"min=1,max=100"`
	Keyword string `query:"keyword"`
}

// CreateCategoryRequest is the payload for POST /doc/category. Bound to
// a dedicated DTO instead of model.DocCategory so we can enforce length
// / required constraints up front — the model has no validate tags and
// would silently accept empty slugs + 5000-char descriptions.
type CreateCategoryRequest struct {
	Slug        string `json:"slug" validate:"required,min=1,max=100"`
	Title       string `json:"title" validate:"required,min=1,max=233"`
	Description string `json:"description" validate:"max=500"`
	Icon        string `json:"icon" validate:"max=200"`
	SortOrder   int    `json:"sort_order" validate:"min=0,max=9999"`
}

// UpdateCategoryRequest is the payload for PUT /doc/category.
type UpdateCategoryRequest struct {
	CategoryID  int    `json:"category_id" validate:"required,min=1"`
	Slug        string `json:"slug" validate:"required,min=1,max=100"`
	Title       string `json:"title" validate:"required,min=1,max=233"`
	Description string `json:"description" validate:"max=500"`
	Icon        string `json:"icon" validate:"max=200"`
	SortOrder   int    `json:"sort_order" validate:"min=0,max=9999"`
}

// DeleteCategoryRequest is the query for DELETE /doc/category.
type DeleteCategoryRequest struct {
	CategoryID int `query:"category_id" validate:"required,min=1"`
}
