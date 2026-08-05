package handler

import (
	"strconv"

	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/role"

	"github.com/gofiber/fiber/v3"
)

// SubmissionHandler exposes the user submission endpoints: file a submission,
// publish a claimable draft, resubmit, withdraw, list your own, and the wizard
// search.
//
// The identity model changed with the registry switchover and it is worth being
// explicit about: the user's OAuth token is no longer forwarded anywhere. kungal
// authenticates its own session and ASSERTS the actor over the Basic-authed S2S
// channel, which is the same posture the editing face already uses — one
// convention for "a product backend acting on behalf of one of its users",
// rather than two.
type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

// claimActor builds the asserted actor for the current session. Trust tier is
// the conservative staff mapping the edit face uses; it is a projection, not an
// authorization gate.
func claimActor(c fiber.Ctx) (catalogclient.EditActor, *errors.AppError) {
	user := middleware.GetUser(c)
	if user == nil {
		return catalogclient.EditActor{}, errors.ErrAuthExpired()
	}
	var tier int16
	if role.CanModerate(user.Roles) {
		tier = 3
	}
	return catalogclient.EditActor{UserID: int64(user.ID), Roles: user.Roles, TrustTier: tier}, nil
}

func submissionGID(c fiber.Ctx) (int, *errors.AppError) {
	gid, err := strconv.Atoi(c.Params("gid"))
	if err != nil || gid <= 0 {
		return 0, errors.ErrBadRequest("无效的 Galgame ID")
	}
	return gid, nil
}

// Submit — POST /api/galgame/submit
func (h *SubmissionHandler) Submit(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var form service.SubmissionForm
	if err := c.Bind().Body(&form); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	res, appErr := h.svc.Submit(c.Context(), actor, &form)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// Claim — POST /api/galgame/:gid/claim. Publishes a draft the registry already
// holds under kungal's id.
func (h *SubmissionHandler) Claim(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, appErr := submissionGID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	res, appErr := h.svc.Claim(c.Context(), actor, gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// Resubmit — POST /api/galgame/:gid/resubmit. Sends a draft or a declined
// submission back to the review queue. The CONTENT of a draft is edited through
// the ordinary editing face, so this endpoint moves state and nothing else.
func (h *SubmissionHandler) Resubmit(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, appErr := submissionGID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	res, appErr := h.svc.Resubmit(c.Context(), actor, gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// Withdraw — DELETE /api/galgame/:gid.
//
// The verb stays DELETE because that is what the action means to the user
// ("撤回我的投稿"), but the registry row survives: an identity is not destroyed
// because a product withdrew its claim. The claim returns to draft and the
// entry can be resubmitted.
func (h *SubmissionHandler) Withdraw(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, appErr := submissionGID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	if _, appErr := h.svc.Withdraw(c.Context(), actor, gid); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "撤回成功")
}

// ListMine — GET /api/galgame/mine
func (h *SubmissionHandler) ListMine(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.svc.ListMine(c.Context(), actor.UserID, collectQuery(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// SearchWithPending — GET /api/galgame/search/wizard
//
// Dedicated endpoint for the 发布向导 flow: one half answers "does this game
// already exist", the other "…and did YOU already submit it". Both are the
// registry's now; the session is what makes the second one personal.
//
// Default search (/api/galgame/search) stays anonymous-only — first-time
// visitors and SSR don't want "突然在首页看到自己的 pending" UX.
func (h *SubmissionHandler) SearchWithPending(c fiber.Ctx) error {
	actor, appErr := claimActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.svc.SearchWithPending(c.Context(), actor.UserID, collectQuery(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}
