package handler

import (
	"kun-galgame-api/internal/friendlink/dto"
	"kun-galgame-api/internal/friendlink/model"
	"kun-galgame-api/internal/friendlink/repository"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/imageclient"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// FriendLinkHandler — admin-managed 友情链接 CRUD + drag-reorder. Pure DB ops,
// no service layer (mirrors UpdateHandler).
type FriendLinkHandler struct {
	repo *repository.FriendLinkRepository
	// cdnBase resolves a stored banner_image_hash into a full CDN URL on read.
	cdnBase string
}

func NewFriendLinkHandler(repo *repository.FriendLinkRepository, cdnBase string) *FriendLinkHandler {
	return &FriendLinkHandler{repo: repo, cdnBase: cdnBase}
}

// List returns all friend links grouped by the 3 fixed categories, each ordered
// by sort_order. Public — rendered on /friend-links and the admin page.
// GET /api/friend-link
func (h *FriendLinkHandler) List(c fiber.Ctx) error {
	// Initialise all 3 keys to non-nil empty slices so the JSON always carries
	// every group (an empty category serialises as [] not null).
	grouped := map[string][]model.FriendLink{
		"official": {},
		"galgame":  {},
		"others":   {},
	}
	for _, fl := range h.repo.FindAllOrdered() {
		if _, ok := grouped[fl.Category]; ok {
			fl.BannerURL = imageclient.ResolveURL(h.cdnBase, fl.BannerImageHash, fl.Banner)
			grouped[fl.Category] = append(grouped[fl.Category], fl)
		}
	}
	return response.OK(c, grouped)
}

// Create adds a friend link, appended to the end of its category.
// POST /api/admin/friend-link
func (h *FriendLinkHandler) Create(c fiber.Ctx) error {
	if _, appErr := middleware.MustGetUser(c); appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.CreateRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	fl := model.FriendLink{
		Category:        req.Category,
		Name:            req.Name,
		Link:            req.Link,
		Description:     req.Description,
		Banner:          req.Banner,
		BannerImageHash: req.BannerImageHash,
		Status:          defaultStatus(req.Status),
	}
	if err := h.repo.Create(&fl); err != nil {
		return response.Error(c, errors.ErrInternal("创建友链失败"))
	}
	return response.OK(c, fl)
}

// Update patches a friend link.
// PUT /api/admin/friend-link
func (h *FriendLinkHandler) Update(c fiber.Ctx) error {
	if _, appErr := middleware.MustGetUser(c); appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	fields := map[string]any{
		"category":          req.Category,
		"name":              req.Name,
		"link":              req.Link,
		"description":       req.Description,
		"banner":            req.Banner,
		"banner_image_hash": req.BannerImageHash,
		"status":            defaultStatus(req.Status),
	}
	if err := h.repo.Update(req.ID, fields); err != nil {
		return response.Error(c, errors.ErrInternal("更新友链失败"))
	}
	return response.OKMessage(c, "友链已更新")
}

// Delete removes a friend link.
// DELETE /api/admin/friend-link?id=
func (h *FriendLinkHandler) Delete(c fiber.Ctx) error {
	if _, appErr := middleware.MustGetUser(c); appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.DeleteRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	h.repo.Delete(req.ID)
	return response.OKMessage(c, "友链已删除")
}

// Reorder persists a new within-category ordering (drag-and-drop result).
// PUT /api/admin/friend-link/reorder
func (h *FriendLinkHandler) Reorder(c fiber.Ctx) error {
	if _, appErr := middleware.MustGetUser(c); appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.ReorderRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if err := h.repo.Reorder(req.Category, req.IDs); err != nil {
		return response.Error(c, errors.ErrInternal("调整友链顺序失败"))
	}
	return response.OKMessage(c, "顺序已保存")
}

func defaultStatus(s string) string {
	if s == "" {
		return "normal"
	}
	return s
}
