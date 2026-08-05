package handler

import (
	"strconv"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// GalgameHandler groups the "core" galgame endpoints: create, detail
// aggregation, list, and local interactions.
type GalgameHandler struct {
	galgameService *service.GalgameService
}

func NewGalgameHandler(galgameService *service.GalgameService) *GalgameHandler {
	return &GalgameHandler{galgameService: galgameService}
}

// The old-wire PR handlers (SubmitPR / MergePR / DeclinePR) retired in E3b —
// every kungal edit write flows through the editing-engine BFF
// (edit_handler.go), which carries their notification / moemoepoint /
// activity side effects forward.

// ──────────────────────────────────────────
// GetDetail / GetList
// ──────────────────────────────────────────

// GetDetail — GET /api/galgame/:gid
//
// Bearer-aware: forwards the caller's OAuth access token (when present
// via OptionalAuth) so the galgame returns the caller's own pending /
// declined drafts in addition to public status=0 rows. Without the
// token, galgame applies its default visibility filter and the call
// behaves identically to the legacy anonymous path.
//
// This is what makes /edit/galgame/draft/:gid (owner viewing own
// pending) and the publish wizard's VNDB-id lookup (claimable VNDB
// draft, status=2) work without dedicated owner-only endpoints.
func (h *GalgameHandler) GetDetail(c fiber.Ctx) error {
	gid, err := strconv.Atoi(c.Params("gid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的 Galgame ID"))
	}

	detail, appErr := h.galgameService.GetDetail(
		c.Context(), gid, optionalUID(c), middleware.GetAccessToken(c), utils.IsSFW(c),
	)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, detail)
}

// GetList — GET /api/galgame
//
// SFW-default. Crawlers and cookie-less visitors get content_limit=sfw
// only; logged-in users with the NSFW switch on see everything. The
// filter happens in the service layer because kungal's galgame table
// has no content_limit field (see service.GetList for the trade-off).
func (h *GalgameHandler) GetList(c fiber.Ctx) error {
	var req dto.GalgameListRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	page, appErr := h.galgameService.GetList(c.Context(), &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// ──────────────────────────────────────────
// Interactions
// ──────────────────────────────────────────

// ToggleLike — PUT /api/galgame/:gid/like
func (h *GalgameHandler) ToggleLike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	gid, err := strconv.Atoi(c.Params("gid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的 Galgame ID"))
	}

	if appErr := h.galgameService.ToggleLike(c.Context(), user.ID, gid); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "操作成功")
}

// Favorite is now collection membership — see GalgameCollectionHandler
// (PUT /galgame/:gid/collections). There is no per-galgame favorite toggle.

// MyInteractions — GET /api/galgame/interactions/mine
// The current user's liked + favorited galgame ids, used to hydrate feed-card
// like/favorite state (the shared feed cache can't carry per-user state).
func (h *GalgameHandler) MyInteractions(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, h.galgameService.GetMyInteractions(user.ID))
}
