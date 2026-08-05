package handler

import (
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// DraftsHandler serves the "未发布的游戏" claim funnel — catalog works no
// product has an entry for yet. Read-only and SFW-default like the rest of the
// galgame surface. See DraftsService.
type DraftsHandler struct {
	draftsService *service.DraftsService
}

func NewDraftsHandler(draftsService *service.DraftsService) *DraftsHandler {
	return &DraftsHandler{draftsService: draftsService}
}

// GetDrafts — GET /api/galgame/drafts?page=&limit=&official_id=&tag_id=&engine_id=
//
// Returns {items, total}: unclaimed VNDB drafts (status=2, newest first) as
// enriched GalgameCard[] so the shared frontend card renders each as a claim
// card. Pagination is mandatory (the SFW draft pool is ~43k rows). The optional
// official_id / tag_id / engine_id scope the list to one taxonomy entity (0 or
// absent = global) — the modal lives on the entity detail pages.
func (h *DraftsHandler) GetDrafts(c fiber.Ctx) error {
	page, limit := parseCollectionPage(c, 24)
	filters := service.DraftFilters{
		// Catalog ids: the entity detail pages moved to that key space (R1).
		// `official_id` keeps its product-facing name — a 会社 is a catalog
		// LABEL, but renaming the query parameter would break every saved link.
		LabelID:  fiber.Query(c, "official_id", 0),
		TagID:    fiber.Query(c, "tag_id", 0),
		EngineID: fiber.Query(c, "engine_id", 0),
		SeriesID: fiber.Query(c, "series_id", 0),

		OriginalLanguages: fiber.Query(c, "original_language", ""),
	}
	pageData, appErr := h.draftsService.GetDrafts(c.Context(), page, limit, filters)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, pageData)
}
