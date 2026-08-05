package handler

import (
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/toolset/dto"
	"kun-galgame-api/internal/toolset/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type PracticalityHandler struct {
	practicalityService *service.PracticalityService
}

func NewPracticalityHandler(practicalityService *service.PracticalityService) *PracticalityHandler {
	return &PracticalityHandler{practicalityService: practicalityService}
}

// GetPracticality returns rating distribution for a toolset.
// GET /api/toolset/:id/practicality
func (h *PracticalityHandler) GetPracticality(c fiber.Ctx) error {
	id := fiber.Params[int](c, "id")
	if id <= 0 {
		return response.Error(c, errors.ErrBadRequest("无效的工具 ID"))
	}

	result := h.practicalityService.GetPracticality(id, optionalUID(c))
	return response.OK(c, result)
}

// UpsertPracticality upserts a user's practicality rating.
// PUT /api/toolset/:id/practicality
func (h *PracticalityHandler) UpsertPracticality(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	id := fiber.Params[int](c, "id")
	if id <= 0 {
		return response.Error(c, errors.ErrBadRequest("无效的工具 ID"))
	}

	var req dto.UpsertPracticalityRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.practicalityService.UpsertPracticality(id, user.ID, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "评分成功")
}

// optionalUID returns the logged-in user's ID from OptionalAuth middleware,
// or 0 if not authenticated.
func optionalUID(c fiber.Ctx) int {
	if user := middleware.GetUser(c); user != nil {
		return user.ID
	}
	return 0
}
