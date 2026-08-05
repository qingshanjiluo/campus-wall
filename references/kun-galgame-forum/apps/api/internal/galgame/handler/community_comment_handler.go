package handler

import (
	"strconv"

	"kun-galgame-api/internal/galgame/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

// CommunityCommentHandler serves the community-backed galgame comment routes
// (the `/comments` plural prefix). Community is now the unconditional comment
// backend (the legacy `/comment` routes were retired in charter step 06a); an
// unconfigured client degrades reads to empty pages and writes to 503.
// Post-addressed writes take the community post id in the path; a `?gid` hint
// carries the galgame id for the LOCAL display counter + mention deep-links
// (the frontend always has it in context).
type CommunityCommentHandler struct {
	service *service.CommunityCommentService
}

func NewCommunityCommentHandler(svc *service.CommunityCommentService) *CommunityCommentHandler {
	return &CommunityCommentHandler{service: svc}
}

// RegisterReads / RegisterWrites mount the community comment routes.
//
// The two halves MUST be mounted on opposite sides of the router's mandatory-
// auth boundary: Fiber middleware is stack-ordered, so a route registered after
// the session-required `Use` passes through it no matter which group handle
// registered it. Mounting the public reads via the optAuth handle but after
// that boundary silently made them login-only (anonymous 205) — the split keeps
// the registration position explicit.
func (h *CommunityCommentHandler) RegisterReads(optAuth fiber.Router) {
	optAuth.Get("/galgame/:gid/comments", h.List)
	optAuth.Get("/galgame/:gid/comments/locate", h.Locate)
}

func (h *CommunityCommentHandler) RegisterWrites(authed fiber.Router) {
	authed.Post("/galgame/:gid/comments", h.Create)
	authed.Put("/galgame/comments/:postId", h.Update)
	authed.Delete("/galgame/comments/:postId", h.Delete)
	authed.Put("/galgame/comments/:postId/like", h.ToggleLike)
	authed.Post("/galgame/comments/:postId/flag", h.Flag)
}

// List returns a flat keyset page of a galgame's comments (optional auth).
// GET /api/galgame/:gid/comments?cursor=<post_number>&limit=<=50
func (h *CommunityCommentHandler) List(c fiber.Ctx) error {
	gid, ok := parsePositive(c.Params("gid"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的 galgame ID"))
	}
	var req struct {
		Cursor string `query:"cursor"`
		Limit  int    `query:"limit" validate:"omitempty,min=1,max=50"`
	}
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.service.GetComments(c.Context(), gid, optionalUID(c), req.Cursor, req.Limit)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// Create posts a comment/reply on a galgame (authenticated).
// POST /api/galgame/:gid/comments   body { content, reply_to_post_id? }
func (h *CommunityCommentHandler) Create(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	gid, ok := parsePositive(c.Params("gid"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的 galgame ID"))
	}
	var req struct {
		Content       string `json:"content" validate:"required,min=1,max=5000"`
		ReplyToPostID *int64 `json:"reply_to_post_id"`
	}
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	item, appErr := h.service.CreateComment(c.Context(), user.ID, gid, req.Content, req.ReplyToPostID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, item)
}

// Update edits a comment's body (author, or moderator via the mod-actor variant).
// PUT /api/galgame/comments/:postId?gid=<id>   body { content }
func (h *CommunityCommentHandler) Update(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	postID, ok := parsePositive64(c.Params("postId"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的评论 ID"))
	}
	var req struct {
		Content string `json:"content" validate:"required,min=1,max=5000"`
	}
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	// This edit route is SHARED across all four comment surfaces — galgame,
	// rating, website, toolset all reuse PUT /galgame/comments/:postId for edits
	// (only delete is per-surface). Each surface has its OWN comment.*.edit key,
	// and runtime per-role/per-user overrides can make those keys diverge, so a
	// union check here would be a hole (revoking one surface's key would not bite
	// on this route). The handler doesn't know the post's surface, so it hands the
	// caller's identity down and the service resolves the surface from the post's
	// anchor before deciding — see UpdateComment.
	item, appErr := h.service.UpdateComment(c.Context(), user.ID, user.Roles, postID, optionalGid(c), req.Content)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, item)
}

// Delete tombstones a comment (author or moderator).
// DELETE /api/galgame/comments/:postId?gid=<id>
func (h *CommunityCommentHandler) Delete(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	postID, ok := parsePositive64(c.Params("postId"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的评论 ID"))
	}
	// Unlike edit, delete is NOT a shared route — the resource surfaces carry
	// their own region-aware delete (ResourceCommentHandler), so this path only
	// ever tombstones galgame (site_game) comments.
	if appErr := h.service.DeleteComment(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.CommentGalgameDelete), postID, optionalGid(c)); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "评论已删除")
}

// ToggleLike flips the like on a comment (the triple write).
// PUT /api/galgame/comments/:postId/like
func (h *CommunityCommentHandler) ToggleLike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	postID, ok := parsePositive64(c.Params("postId"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的评论 ID"))
	}
	result, appErr := h.service.ToggleLike(c.Context(), user.ID, postID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// Flag reports a comment to the community moderation queue.
// POST /api/galgame/comments/:postId/flag   body { reason 0-4, note? }
func (h *CommunityCommentHandler) Flag(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	postID, ok := parsePositive64(c.Params("postId"))
	if !ok {
		return response.Error(c, errors.ErrBadRequest("非法的评论 ID"))
	}
	var req struct {
		Reason int    `json:"reason" validate:"min=0,max=4"`
		Note   string `json:"note" validate:"omitempty,max=500"`
	}
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.service.FlagComment(c.Context(), user.ID, postID, req.Reason, req.Note); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "举报已提交")
}

// Locate resolves an old galgame_comment id to its migrated community post.
// GET /api/galgame/:gid/comments/locate?legacy_id=<int>
func (h *CommunityCommentHandler) Locate(c fiber.Ctx) error {
	var req struct {
		LegacyID int `query:"legacy_id" validate:"required,min=1"`
	}
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	result, appErr := h.service.Locate(req.LegacyID)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// optionalGid reads the ?gid hint (the galgame id the frontend renders the
// comment under) for local counter + mention-deeplink bookkeeping on the
// post-addressed write routes; nil when absent/invalid (the enrichment is then
// skipped and the drift tolerated).
func optionalGid(c fiber.Ctx) *int {
	if gid, ok := parsePositive(c.Query("gid")); ok {
		return &gid
	}
	return nil
}

func parsePositive(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func parsePositive64(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
