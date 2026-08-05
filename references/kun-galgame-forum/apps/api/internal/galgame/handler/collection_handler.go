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

// GalgameCollectionHandler serves the galgame collection (收藏夹) endpoints:
// CRUD, the collection-picker read/write, per-user collection list, and detail.
type GalgameCollectionHandler struct {
	collectionService *service.CollectionService
}

func NewGalgameCollectionHandler(collectionService *service.CollectionService) *GalgameCollectionHandler {
	return &GalgameCollectionHandler{collectionService: collectionService}
}

// Create — POST /api/galgame/collection
func (h *GalgameCollectionHandler) Create(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.CreateCollectionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	id, appErr := h.collectionService.Create(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, id)
}

// Update — PATCH /api/galgame/collection/:cid
func (h *GalgameCollectionHandler) Update(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	cid, err := strconv.Atoi(c.Params("cid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的收藏夹 ID"))
	}
	var req dto.UpdateCollectionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.collectionService.Update(c.Context(), user.ID, cid, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "收藏夹已更新")
}

// Delete — DELETE /api/galgame/collection/:cid
func (h *GalgameCollectionHandler) Delete(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	cid, err := strconv.Atoi(c.Params("cid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的收藏夹 ID"))
	}
	if appErr := h.collectionService.Delete(user.ID, cid); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "收藏夹已删除")
}

// GetDetail — GET /api/galgame/collection/:cid (optional auth for access + is_owner)
func (h *GalgameCollectionHandler) GetDetail(c fiber.Ctx) error {
	cid, err := strconv.Atoi(c.Params("cid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的收藏夹 ID"))
	}
	page, limit := parseCollectionPage(c, 24)
	detail, appErr := h.collectionService.GetDetail(
		c.Context(), optionalUID(c), cid, page, limit, utils.IsSFW(c),
	)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, detail)
}

// MyCollectionsForGalgame — GET /api/galgame/:gid/collections/mine
// Powers the picker modal; lazily creates the default collection.
func (h *GalgameCollectionHandler) MyCollectionsForGalgame(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, err := strconv.Atoi(c.Params("gid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的 Galgame ID"))
	}
	cols, appErr := h.collectionService.GetMyCollectionsForGalgame(user.ID, gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"collections": cols})
}

// SetMembership — PUT /api/galgame/:gid/collections
// Body: { "collection_ids": [1, 5] } — the full desired membership.
func (h *GalgameCollectionHandler) SetMembership(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, err := strconv.Atoi(c.Params("gid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的 Galgame ID"))
	}
	var req dto.SetCollectionMembershipRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.collectionService.SetMembership(c.Context(), user.ID, gid, req.CollectionIDs); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "操作成功")
}

// GetUserCollections — GET /api/user/:id/collections (optional auth for visibility)
func (h *GalgameCollectionHandler) GetUserCollections(c fiber.Ctx) error {
	ownerID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的用户 ID"))
	}
	page, limit := parseCollectionPage(c, 24)
	items, total, appErr := h.collectionService.ListForUser(
		c.Context(), ownerID, optionalUID(c), page, limit, utils.IsSFW(c),
	)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.Paginated(c, items, total)
}

// parseCollectionPage reads page/limit with sane defaults (page>=1, 1<=limit<=50).
func parseCollectionPage(c fiber.Ctx, defLimit int) (page, limit int) {
	page, _ = strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	limit, _ = strconv.Atoi(c.Query("limit"))
	if limit < 1 {
		limit = defLimit
	}
	if limit > 50 {
		limit = 50
	}
	return
}
