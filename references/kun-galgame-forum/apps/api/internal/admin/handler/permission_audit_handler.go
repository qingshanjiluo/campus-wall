package handler

import (
	"strconv"

	"kun-galgame-api/internal/admin/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// PermissionAuditHandler serves the append-only permission-override audit log.
type PermissionAuditHandler struct {
	svc *service.PermissionAuditService
}

func NewPermissionAuditHandler(svc *service.PermissionAuditService) *PermissionAuditHandler {
	return &PermissionAuditHandler{svc: svc}
}

// List returns a page of the audit log (newest first). page/limit are sanitized
// by the service (page ≥ 1; limit default 30, capped at 100).
// GET /api/admin/permission-audit?page=&limit=
func (h *PermissionAuditHandler) List(c fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.svc.List(c.Context(), page, limit)
	if err != nil {
		return response.Error(c, errors.ErrInternal("获取权限审计日志失败"))
	}
	return response.OK(c, res)
}
