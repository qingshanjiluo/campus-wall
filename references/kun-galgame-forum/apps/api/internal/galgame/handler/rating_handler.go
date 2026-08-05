package handler

import (
	"strconv"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type RatingHandler struct {
	ratingService *service.RatingService
}

func NewRatingHandler(ratingService *service.RatingService) *RatingHandler {
	return &RatingHandler{ratingService: ratingService}
}

// GetAllRatings returns paginated galgame ratings.
// GET /api/galgame-rating/all
// GetAllRatings — GET /api/galgame-rating/all
//
// SFW-default: anonymous + cookie-less requests get only ratings whose
// galgame is content_limit=sfw.
func (h *RatingHandler) GetAllRatings(c fiber.Ctx) error {
	var req dto.RatingListRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	page, appErr := h.ratingService.GetAllRatings(c.Context(), &req, utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetRatingDetail returns a single rating with comments, liked users, and galgame.
// GET /api/galgame-rating/:id
func (h *RatingHandler) GetRatingDetail(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的评分 ID"))
	}

	currentUID := optionalUID(c)
	detail, appErr := h.ratingService.GetRatingDetail(c.Context(), id, currentUID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, detail)
}

// CreateRating — POST /api/galgame-rating
func (h *RatingHandler) CreateRating(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.CreateRatingRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	created, appErr := h.ratingService.CreateRating(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, created)
}

// UpdateRating — PUT /api/galgame-rating/:id
func (h *RatingHandler) UpdateRating(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateRatingRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.ratingService.UpdateRating(c.Context(), user.ID, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "评分更新成功")
}

// DeleteRating — DELETE /api/galgame-rating/:id
func (h *RatingHandler) DeleteRating(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.DeleteRatingRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.ratingService.DeleteRating(user.ID, perm.CanUser(user.ID, user.Roles, perm.RatingDeleteAny), req.GalgameRatingID); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "评分已删除")
}

// ToggleLike — PUT /api/galgame-rating/:id/like
func (h *RatingHandler) ToggleLike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.ToggleRatingLikeRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.ratingService.ToggleRatingLike(user.ID, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "操作成功")
}

// The rating comment WRITE routes (POST/PUT/DELETE /galgame-rating/:id/comment)
// were retired when the resource comment areas moved onto the community
// primitive (charter step 06a). Comments are now created via the community-
// backed /galgame-rating/:id/comments routes (ResourceCommentHandler); the
// rating DETAIL response still embeds the legacy comment list for now (a later
// wave adapts that reader).
