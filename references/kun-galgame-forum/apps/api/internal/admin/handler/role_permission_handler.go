package handler

import (
	"kun-galgame-api/internal/admin/dto"
	"kun-galgame-api/internal/admin/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// RolePermissionHandler serves the admin role→permission matrix (read + replace)
// and the public effective-bundles read.
type RolePermissionHandler struct {
	svc *service.RolePermissionService
}

func NewRolePermissionHandler(svc *service.RolePermissionService) *RolePermissionHandler {
	return &RolePermissionHandler{svc: svc}
}

// GetMatrix returns the full role→permission matrix for the admin editor.
// GET /api/admin/role-permissions
func (h *RolePermissionHandler) GetMatrix(c fiber.Ctx) error {
	matrix, err := h.svc.Matrix(c.Context())
	if err != nil {
		return response.Error(c, errors.ErrInternal("获取角色权限矩阵失败"))
	}
	return response.OK(c, matrix)
}

// Replace atomically replaces the path role's override set and returns the fresh
// matrix. PUT /api/admin/role-permissions/:role
func (h *RolePermissionHandler) Replace(c fiber.Ctx) error {
	operator, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.ReplaceOverridesRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	matrix, appErr := h.svc.ReplaceOverrides(c.Context(), operator.ID, operator.Roles, c.Params("role"), req.Overrides)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, matrix)
}

// GetBundles returns the EFFECTIVE role→permission bundles for
// creator/moderator/admin/ren. PUBLIC (no auth): it only tells the FE which
// capabilities a role holds so it can show/hide UI affordances — it is
// non-sensitive config and grants nothing.
// GET /api/perm/bundles
func (h *RolePermissionHandler) GetBundles(c fiber.Ctx) error {
	bundles := perm.EffectiveBundles()
	out := make(map[string][]string, len(bundles))
	for role, perms := range bundles {
		keys := make([]string, len(perms))
		for i, p := range perms {
			keys[i] = string(p)
		}
		out[role] = keys
	}
	return response.OK(c, out)
}
