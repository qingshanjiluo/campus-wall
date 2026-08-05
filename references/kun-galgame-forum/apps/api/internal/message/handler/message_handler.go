package handler

import (
	"strconv"

	"kun-galgame-api/internal/message/dto"
	"kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type MessageHandler struct {
	messageService *service.MessageService
}

func NewMessageHandler(messageService *service.MessageService) *MessageHandler {
	return &MessageHandler{messageService: messageService}
}

// GetMessages returns paginated notification messages.
// GET /api/message
func (h *MessageHandler) GetMessages(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.ListMessagesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	result, appErr := h.messageService.GetMessages(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// GetMutedMessages returns messages of muted categories (the "已静音的消息"
// view). Optional ?type= narrows to a single currently-muted category.
// GET /api/message/muted
func (h *MessageHandler) GetMutedMessages(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.ListMessagesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	result, appErr := h.messageService.GetMutedMessages(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// DeleteMessage deletes a single notification message.
// DELETE /api/message/:id
func (h *MessageHandler) DeleteMessage(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的消息 ID"))
	}

	if appErr := h.messageService.DeleteMessage(c.Context(), user.ID, id); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "消息已删除")
}

// GetSystemMessages returns admin broadcast messages with this user's
// per-user `isRead` flag (computed against the HWM cursor in
// system_message_read_state — see migration 012).
//
// Requires auth: unread/read is meaningful only with a known user.
// GET /api/message/admin
func (h *MessageHandler) GetSystemMessages(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	messages, appErr := h.messageService.GetSystemMessages(c.Context(), user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, messages)
}

// MarkAdminRead advances the caller's HWM cursor to MAX(system_message.id)
// so every existing broadcast becomes read for this user only — no fan-out
// to other users, fixed in migration 012.
// PUT /api/message/admin/read
func (h *MessageHandler) MarkAdminRead(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.messageService.MarkAllSystemRead(c.Context(), user.ID); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "已标记全部已读")
}

// GetNavSummary returns nav-bar summary [notice, system].
// GET /api/message/nav/system
func (h *MessageHandler) GetNavSummary(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	result, appErr := h.messageService.GetNavSummary(c.Context(), user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// MarkAllRead marks all user notification messages as read.
// PUT /api/message/system/read
func (h *MessageHandler) MarkAllRead(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.messageService.MarkAllRead(c.Context(), user.ID); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "已标记全部已读")
}
