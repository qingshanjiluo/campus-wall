package handler

import (
	"kun-galgame-api/internal/search/dto"
	"kun-galgame-api/internal/search/service"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type SearchHandler struct {
	searchService *service.SearchService
}

func NewSearchHandler(searchService *service.SearchService) *SearchHandler {
	return &SearchHandler{searchService: searchService}
}

// Search performs keyword search across different content types.
// GET /api/search
func (h *SearchHandler) Search(c fiber.Ctx) error {
	var req dto.SearchRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	switch req.Type {
	case "topic":
		res, appErr := h.searchService.SearchTopics(c.Context(), req.Keywords, req.Page, req.Limit)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, res.Items, res.Total)
	case "galgame":
		// Search ignores the SFW cookie by product decision: anyone (logged
		// in or not, SFW-mode or not) can discover every game by name. The
		// SFW gate is enforced on the detail page itself (anonymous SFW
		// callers see a click-to-confirm interstitial there). Pass
		// isSFW=false so galgame receives content_limit=all.
		res, appErr := h.searchService.SearchGalgames(
			c.Context(), req.Keywords, req.Page, req.Limit, false,
		)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, res.Items, res.Total)
	case "user":
		res, appErr := h.searchService.SearchUsers(c.Context(), req.Keywords, req.Page, req.Limit)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, res.Items, res.Total)
	case "reply":
		res, appErr := h.searchService.SearchReplies(c.Context(), req.Keywords, req.Page, req.Limit)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, res.Items, res.Total)
	case "comment":
		res, appErr := h.searchService.SearchComments(c.Context(), req.Keywords, req.Page, req.Limit)
		if appErr != nil {
			return response.Error(c, appErr)
		}
		return response.Paginated(c, res.Items, res.Total)
	default:
		return response.OK(c, []any{})
	}
}
