package handler

import (
	"strings"

	"kun-galgame-api/internal/activity/dto"
	"kun-galgame-api/internal/activity/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type ActivityHandler struct {
	activityService *service.ActivityService
}

func NewActivityHandler(activityService *service.ActivityService) *ActivityHandler {
	return &ActivityHandler{activityService: activityService}
}

// GetActivity returns activity feed filtered by type.
// GET /api/activity
func (h *ActivityHandler) GetActivity(c fiber.Ctx) error {
	var req dto.ActivityRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	res, appErr := h.activityService.GetActivity(c.Context(), req.Type, req.Cursor, req.Limit, utils.IsSFW(c), req.ShowNoResource)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// GetTab returns one of the home-page feed's five tab buckets.
// GET /api/activity/tab
func (h *ActivityHandler) GetTab(c fiber.Ctx) error {
	var req dto.TabRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	// The 全部 tab forces SFW (req.ForceSfw) so NSFW topics + galgame activity are
	// filtered out of the main stream even for NSFW-enabled viewers.
	isSFW := utils.IsSFW(c) || req.ForceSfw

	// Configurable tab → the FE sends its selected kind set; otherwise fall back
	// to a legacy built-in bucket (tab=all/topic/galgame/resource/others).
	var res *service.Result
	var appErr *errors.AppError
	if req.Types != "" {
		res, appErr = h.activityService.GetFeedByTypes(c.Context(), strings.Split(req.Types, ","), req.Cursor, req.Limit, isSFW, req.ShowNoResource)
	} else {
		res, appErr = h.activityService.GetTab(c.Context(), req.Tab, req.Cursor, req.Limit, isSFW, req.ShowNoResource)
	}
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}

// GetTimeline returns mixed activity timeline.
// GET /api/activity/timeline
func (h *ActivityHandler) GetTimeline(c fiber.Ctx) error {
	var req dto.TimelineRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	res, appErr := h.activityService.GetTimeline(c.Context(), req.Cursor, req.Limit, utils.IsSFW(c), req.ShowNoResource)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, res)
}
