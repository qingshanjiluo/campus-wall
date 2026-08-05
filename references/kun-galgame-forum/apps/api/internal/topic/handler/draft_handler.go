package handler

import (
	"strconv"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/topic/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type TopicDraftHandler struct {
	draftService *service.DraftService
}

func NewTopicDraftHandler(draftService *service.DraftService) *TopicDraftHandler {
	return &TopicDraftHandler{draftService: draftService}
}

// Save stores the current editor state as a new draft.
// POST /api/topic/draft
func (h *TopicDraftHandler) Save(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.SaveTopicDraftRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	id, appErr := h.draftService.Save(user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, id)
}

// List returns the current user's drafts (metadata only), newest first.
// GET /api/topic/draft
func (h *TopicDraftHandler) List(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	items, appErr := h.draftService.List(user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, items)
}

// Get returns one draft in full, to restore into the editor.
// GET /api/topic/draft/:id
func (h *TopicDraftHandler) Get(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的草稿 ID"))
	}

	detail, appErr := h.draftService.Get(user.ID, id)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, detail)
}

// Delete removes one of the current user's drafts.
// DELETE /api/topic/draft/:id
func (h *TopicDraftHandler) Delete(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的草稿 ID"))
	}

	if appErr := h.draftService.Delete(user.ID, id); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "草稿已删除")
}
