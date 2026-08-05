package handler

import (
	"strconv"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/topic/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type ReplyHandler struct {
	replyService *service.ReplyService
}

func NewReplyHandler(replyService *service.ReplyService) *ReplyHandler {
	return &ReplyHandler{replyService: replyService}
}

// GetReplies returns paginated reply list for a topic.
// GET /api/topic/:tid/reply
func (h *ReplyHandler) GetReplies(c fiber.Ctx) error {
	var req dto.ListRepliesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	userInfo := middleware.GetUser(c)

	replies, appErr := h.replyService.GetReplies(c.Context(), &req, userInfo)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, replies)
}

// GetReplyDetail returns a single reply with full details.
// GET /api/topic/:tid/reply/detail
func (h *ReplyHandler) GetReplyDetail(c fiber.Ctx) error {
	replyID, err := strconv.Atoi(c.Query("replyId"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的回复 ID"))
	}

	userInfo := middleware.GetUser(c)

	detail, appErr := h.replyService.GetReplyDetail(c.Context(), replyID, userInfo)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, detail)
}

// GetReplyLocate resolves a deep-link target (a reply floor or a comment id) to
// the reply-stream page it lives on, so the frontend can load that page directly
// and scroll to it.
// GET /api/topic/:tid/reply/locate?reply=<floor> | ?comment=<commentId>
func (h *ReplyHandler) GetReplyLocate(c fiber.Ctx) error {
	topicID, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}
	floor, _ := strconv.Atoi(c.Query("reply"))
	commentID, _ := strconv.Atoi(c.Query("comment"))
	if floor <= 0 && commentID <= 0 {
		return response.Error(c, errors.ErrBadRequest("缺少 reply 或 comment 参数"))
	}

	res, appErr := h.replyService.LocateReply(topicID, floor, commentID, 30)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, res)
}

// CreateReply creates a new reply to a topic.
// POST /api/topic/:tid/reply
func (h *ReplyHandler) CreateReply(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.CreateReplyRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	reply, appErr := h.replyService.CreateReply(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, reply)
}

// UpdateReply edits an existing reply.
// PUT /api/topic/:tid/reply
func (h *ReplyHandler) UpdateReply(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.UpdateReplyRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	_ = user

	if appErr := h.replyService.UpdateReply(c.Context(), user.ID, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "回复更新成功")
}

// DeleteReply deletes a reply with cascade.
// DELETE /api/topic/:tid/reply
func (h *ReplyHandler) DeleteReply(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	replyID, err := strconv.Atoi(c.Query("replyId"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的回复 ID"))
	}

	if appErr := h.replyService.DeleteReply(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.ReplyDeleteAny), replyID); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "回复已删除")
}

// ToggleReplyLike toggles like on a reply.
// PUT /api/topic/:tid/reply/like
func (h *ReplyHandler) ToggleReplyLike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.ReplyInteractionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.replyService.ToggleReplyLike(c.Context(), user.ID, req.ReplyID); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// ToggleReplyDislike toggles dislike on a reply.
// PUT /api/topic/:tid/reply/dislike
func (h *ReplyHandler) ToggleReplyDislike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.ReplyInteractionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.replyService.ToggleReplyDislike(c.Context(), user.ID, req.ReplyID); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// ToggleReplyReaction adds/removes a reaction (like/dislike/emoji) on a reply.
// PUT /api/topic/:tid/reply/reaction
func (h *ReplyHandler) ToggleReplyReaction(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.ReplyReactionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.replyService.ToggleReplyReaction(c.Context(), user.ID, req.ReplyID, req.Reaction); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// PinReply toggles pinning a reply.
// PUT /api/topic/:tid/reply/pin
func (h *ReplyHandler) PinReply(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	var req dto.PinReplyRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.replyService.PinReply(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.ReplyPin), tid, req.ReplyID); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}
