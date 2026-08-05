package handler

import (
	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

// GalgameProxyHandler groups galgame pass-through endpoints and the galgame sub-routes
// that proxy to galgame + enrich with local user data.
type GalgameProxyHandler struct {
	galgameProxyService *service.GalgameProxyService
}

func NewGalgameProxyHandler(galgameProxyService *service.GalgameProxyService) *GalgameProxyHandler {
	return &GalgameProxyHandler{galgameProxyService: galgameProxyService}
}

// The generic write proxy (ProxyWriteWithToken → wiki taxonomy writes)
// retired in wave 169 with the taxonomy staff lane — its upstream faces
// 404 since wave-161 P5.

// ──────────────────────────────────────────
// Galgame sub-routes
// ──────────────────────────────────────────

// GetGalgameLinks — GET /galgame/:gid/link/all
func (h *GalgameProxyHandler) GetGalgameLinks(c fiber.Ctx) error {
	links, appErr := h.galgameProxyService.GetGalgameLinks(c.Context(), c.Params("gid"))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, links)
}

// The old-wire revision/PR reads (GetGalgameHistory / GetGalgamePRs and the
// bare ProxyGet they rode with) retired in E3b — kungal's history, diff and
// proposal reads all flow through the editing-engine BFF (edit_handler.go).
