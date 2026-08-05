package handler

import (
	"strconv"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/user/dto"
	"kun-galgame-api/internal/user/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// UserHandler exposes the kungal-side user-facing endpoints. After the
// OAuth-as-truth migration, kungal no longer brokers identity changes
// (bio / username / email / avatar / ban / delete) — those live in the
// OAuth admin UI. The endpoints here only deal with kungal-specific
// state (check-in / moemoepoint / unread counts) and content listings
// keyed by user_id.
type UserHandler struct {
	userService        *service.UserService
	userContentService *service.UserContentService
}

func NewUserHandler(
	userService *service.UserService,
	userContentService *service.UserContentService,
) *UserHandler {
	return &UserHandler{
		userService:        userService,
		userContentService: userContentService,
	}
}

// GetProfile returns a user's public profile (identity from OAuth, stats
// from kungal local).
// GET /api/user/:userID
func (h *UserHandler) GetProfile(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	profile, appErr := h.userService.GetUserProfile(c.Context(), userID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, profile)
}

// CheckIn handles daily check-in.
// POST /api/user/check-in
func (h *UserHandler) CheckIn(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	points, appErr := h.userService.CheckIn(c.Context(), user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, points)
}

// GetStatus returns the user's status (moemoepoints, check-in, unread messages).
// GET /api/user/status
func (h *UserHandler) GetStatus(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	status, appErr := h.userService.GetUserStatus(c.Context(), user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, status)
}

// GetNotificationPreferences returns the caller's muted notification categories
// (the opt-out set that suppresses the red dot / unread badges).
// GET /api/user/notification-preferences
func (h *UserHandler) GetNotificationPreferences(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	prefs, appErr := h.userService.GetNotificationPreferences(user.ID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, prefs)
}

// UpdateNotificationPreferences replaces the caller's muted set. Unknown keys
// are dropped server-side. Returns the sanitized set.
// PUT /api/user/notification-preferences body {muted_types}
func (h *UserHandler) UpdateNotificationPreferences(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateNotificationPreferenceRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	prefs, appErr := h.userService.UpdateNotificationPreferences(user.ID, req.MutedTypes)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, prefs)
}

// GetMoemoepointLog returns the session user's unified moemoepoint ledger
// (cursor-paginated, newest first). Scoped to the caller — there is no :id
// param, so a user can only read their OWN ledger.
// GET /api/user/moemoepoint/log?limit=&before_id=&reason=
func (h *UserHandler) GetMoemoepointLog(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	limit := fiber.Query[int](c, "limit", 20)
	if limit < 1 || limit > 50 {
		limit = 20
	}
	beforeID := max(fiber.Query[int](c, "before_id", 0), 0)
	page, appErr := h.userService.GetMoemoepointLog(
		c.Context(), user.ID, limit, beforeID, c.Query("reason"))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// SearchMention returns up to a handful of users matching `q`, for the editor's
// @mention autocomplete dropdown. Behind userAuth (only logged-in users
// compose), so no user lookup is needed here — just proxy the query.
// GET /api/user/search?q=&limit=
func (h *UserHandler) SearchMention(c fiber.Ctx) error {
	users, appErr := h.userService.SearchMentionUsers(
		c.Context(), c.Query("q"), fiber.Query[int](c, "limit", 8))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, users)
}

// GetFloatingCard returns lightweight user info for hover card.
// GET /api/user/:userID/floating — target user is read from ?userId=N (legacy).
func (h *UserHandler) GetFloatingCard(c fiber.Ctx) error {
	var req dto.FloatingCardRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	card, appErr := h.userService.GetFloatingCard(c.Context(), req.UserID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, card)
}

// GetUserGalgames returns a user's galgame list.
// GET /api/user/:userID/galgames
func (h *UserHandler) GetUserGalgames(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserGalgamesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	cards, total, appErr := h.userContentService.GetUserGalgameCards(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.Paginated(c, cards, total)
}

// GetUserTopics returns a user's topic list.
// GET /api/user/:userID/topics
func (h *UserHandler) GetUserTopics(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserTopicsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	items, total, appErr := h.userContentService.GetUserTopics(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"topics": items, "total": total})
}

// GetUserReplies returns a user's reply list.
// GET /api/user/:userID/replies
func (h *UserHandler) GetUserReplies(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserRepliesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	items, total, appErr := h.userContentService.GetUserReplies(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"replies": items, "total": total})
}

// GetUserComments returns a user's comment list.
// GET /api/user/:userID/comments
func (h *UserHandler) GetUserComments(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserCommentsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	items, total, appErr := h.userContentService.GetUserComments(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"comments": items, "total": total})
}

// GetUserResources returns a user's galgame resource list.
// GET /api/user/:userID/resources
func (h *UserHandler) GetUserResources(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserResourcesRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.userContentService.GetUserResources(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetUserGalgameComments returns the "评论 / 被评论 / 点赞评论" comment
// rows for the three sub-tabs under /user/:id/galgame/. Replaces the
// old behaviour where these tabs surfaced the parent galgames as
// galgame-cards — what the user actually wanted was the comment-card
// view used by /user/:id/comment/.
// GET /api/user/:userID/galgame-comments
func (h *UserHandler) GetUserGalgameComments(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserGalgameCommentsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	items, nextCursor, appErr := h.userContentService.GetUserGalgameComments(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"comments": items, "next_cursor": nextCursor})
}

// GetUserRatings returns a user's galgame rating list.
// GET /api/user/:userID/ratings
func (h *UserHandler) GetUserRatings(c fiber.Ctx) error {
	userID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	var req dto.UserRatingsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.userContentService.GetUserRatings(c.Context(), userID, &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}
