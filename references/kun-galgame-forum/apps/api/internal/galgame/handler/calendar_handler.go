package handler

import (
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// CalendarHandler serves the galgame release-calendar endpoints. Read-only and
// SFW-default like the rest of the galgame surface — utils.IsSFW reads the
// content-rating cookie, and the service maps it to the galgame content_limit
// (sfw / all). See service.CalendarService.
type CalendarHandler struct {
	calendarService *service.CalendarService
}

func NewCalendarHandler(calendarService *service.CalendarService) *CalendarHandler {
	return &CalendarHandler{calendarService: calendarService}
}

// GetMonth — GET /api/galgame/calendar?month=YYYY-MM
func (h *CalendarHandler) GetMonth(c fiber.Ctx) error {
	page, appErr := h.calendarService.GetMonth(c.Context(), collectQuery(c), utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetPending — GET /api/galgame/calendar/pending?year=YYYY
func (h *CalendarHandler) GetPending(c fiber.Ctx) error {
	page, appErr := h.calendarService.GetPending(c.Context(), collectQuery(c), utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetTBA — GET /api/galgame/calendar/tba
func (h *CalendarHandler) GetTBA(c fiber.Ctx) error {
	page, appErr := h.calendarService.GetTBA(c.Context(), collectQuery(c), utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetUpcoming — GET /api/galgame/calendar/upcoming
func (h *CalendarHandler) GetUpcoming(c fiber.Ctx) error {
	page, appErr := h.calendarService.GetUpcoming(c.Context(), utils.IsSFW(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}
