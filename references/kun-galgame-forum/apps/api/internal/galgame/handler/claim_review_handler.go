package handler

import (
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// ClaimReviewHandler is the moderation face of the submission lifecycle.
//
// It replaces the wiki admin message queue plus its status-change proxy. The
// route gate stays RequireModerator — the forum's own entry check — while the
// registry re-checks the asserted actor against catalog.claim.review, which
// wave 157 granted moderator and up precisely so this transition changes no
// one's rights.
type ClaimReviewHandler struct {
	svc *service.ClaimReviewService
}

func NewClaimReviewHandler(svc *service.ClaimReviewService) *ClaimReviewHandler {
	return &ClaimReviewHandler{svc: svc}
}

// PendingQueue — GET /api/admin/galgame/submissions (moderator+).
func (h *ClaimReviewHandler) PendingQueue(c fiber.Ctx) error {
	page, appErr := h.svc.PendingQueue(c.Context(), collectQuery(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// reviewRequest is one verdict. Reason is required by decline and recorded on
// the event either way — it is what reaches the submitter.
type reviewRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// Review — POST /api/admin/galgame/:gid/review (moderator+).
//
// The old surface was PUT .../status with an integer. There is no status to
// PUT any more: the verdict is the vocabulary, which is what makes "who decided
// this, and why" a recorded fact rather than something a reviewer had to
// remember to write down somewhere else.
func (h *ClaimReviewHandler) Review(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, appErr := submissionGID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req reviewRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	res, appErr := h.svc.Review(c.Context(), actor, gid, req.Action, req.Reason)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}
