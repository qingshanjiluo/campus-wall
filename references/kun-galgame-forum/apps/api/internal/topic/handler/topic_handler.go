package handler

import (
	"strconv"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/topic/service"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type TopicHandler struct {
	topicService      *service.TopicService
	topicWriteService *service.TopicWriteService
}

func NewTopicHandler(
	topicService *service.TopicService,
	topicWriteService *service.TopicWriteService,
) *TopicHandler {
	return &TopicHandler{
		topicService:      topicService,
		topicWriteService: topicWriteService,
	}
}

// MyInteractions returns the viewer's favorited topic ids + reactions, to
// hydrate the feed card's 收藏 + reaction state (the shared feed can't carry it).
// GET /api/topic/interactions/mine
func (h *TopicHandler) MyInteractions(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, h.topicService.GetMyInteractions(user.ID))
}

// GetList returns paginated topic list.
// GET /api/topic
//
// SFW default: anonymous crawlers and users without the NSFW cookie set
// see only `is_nsfw = false` rows. Logged-in users (or anyone) who flip
// the NSFW switch in client settings (Pinia-persisted cookie
// `KUNGalgameSettings.showKUNGalgameContentLimit = "nsfw"`) get the full
// list. Search engines that don't carry cookies always land on SFW.
func (h *TopicHandler) GetList(c fiber.Ctx) error {
	var req dto.ListTopicsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if req.SortField == "" {
		req.SortField = "status_update_time"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// list_repo flips on `isNSFW = true` (no filter) vs `false` (where
	// is_nsfw=false). utils.IsSFW returns true when the user wants SFW,
	// so the service parameter is the negation.
	isNSFW := !utils.IsSFW(c)

	items, total, appErr := h.topicService.GetList(c.Context(), &req, isNSFW)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, dto.TopicListResponse{Topics: items, Total: total})
}

// GetResourceList returns topics filtered to resource sections (g-seeking, g-other, t-help).
// GET /api/resource
//
// Same SFW-default cookie semantics as GetList.
func (h *TopicHandler) GetResourceList(c fiber.Ctx) error {
	var req dto.ListTopicsRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if req.SortField == "" {
		req.SortField = "status_update_time"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	isNSFW := !utils.IsSFW(c)
	items, _, appErr := h.topicService.GetResourceList(c.Context(), &req, isNSFW)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, items)
}

// GetDetail returns a single topic with all associated data.
// GET /api/topic/:tid
//
// NSFW is NOT gated server-side — by design, mirroring the galgame
// detail policy (see GalgameService.GetDetail). FE shows a "click to
// confirm" interstitial for anonymous + SFW-cookie callers landing on
// an is_nsfw topic; logged-in or NSFW-mode callers see it directly.
// SEO meta is also suppressed by FE useKunDisableSeo on NSFW pages.
func (h *TopicHandler) GetDetail(c fiber.Ctx) error {
	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	userInfo := middleware.GetUser(c)

	detail, appErr := h.topicService.GetDetail(c.Context(), tid, userInfo)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, detail)
}

// GetTopicReactionHistory lists a topic's reaction events (newest first) for the
// 查看历史 modal — reactor + reaction key + time.
// GET /api/topic/:tid/reaction/history
func (h *TopicHandler) GetTopicReactionHistory(c fiber.Ctx) error {
	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	records, appErr := h.topicService.GetTopicReactionHistory(c.Context(), tid)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, records)
}

// Create creates a new topic.
// POST /api/topic
func (h *TopicHandler) Create(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	var req dto.CreateTopicRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	topicID, appErr := h.topicWriteService.Create(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, topicID)
}

// Update edits an existing topic.
// PUT /api/topic/:tid
func (h *TopicHandler) Update(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无��的话题 ID"))
	}

	var req dto.UpdateTopicRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.topicWriteService.Update(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.TopicEditAny), tid, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "话题更新成功")
}

// ToggleLike toggles like on a topic.
// PUT /api/topic/:tid/like
func (h *TopicHandler) ToggleLike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	if appErr := h.topicWriteService.ToggleLike(c.Context(), user.ID, tid); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// ToggleReaction adds/removes a reaction (like/dislike/emoji) on a topic.
// PUT /api/topic/:tid/reaction
func (h *TopicHandler) ToggleReaction(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	var req dto.ReactionRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.topicWriteService.ToggleReaction(c.Context(), user.ID, tid, req.Reaction); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// ToggleDislike toggles dislike on a topic.
// PUT /api/topic/:tid/dislike
func (h *TopicHandler) ToggleDislike(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无��的话题 ID"))
	}

	if appErr := h.topicWriteService.ToggleDislike(c.Context(), user.ID, tid); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "��作成功")
}

// Upvote pushes a topic (costs sender moemoepoints).
// PUT /api/topic/:tid/upvote
func (h *TopicHandler) Upvote(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效��话�� ID"))
	}

	// Optional "why I pushed it" one-liner (<=30 chars, capped server-side);
	// an absent / non-JSON body is fine — it just means no description.
	var body struct {
		Description string `json:"description"`
	}
	_ = c.Bind().Body(&body)

	if appErr := h.topicWriteService.Upvote(c.Context(), user.ID, tid, body.Description); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "推话题成功")
}

// GetUpvotes returns a topic's 推话题 records — who pushed it + their one-liner.
// GET /api/topic/:tid/upvotes
func (h *TopicHandler) GetUpvotes(c fiber.Ctx) error {
	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	records, appErr := h.topicService.GetTopicUpvotes(c.Context(), tid)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OK(c, records)
}

// ToggleFavorite toggles favorite on a topic.
// PUT /api/topic/:tid/favorite
func (h *TopicHandler) ToggleFavorite(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话题 ID"))
	}

	if appErr := h.topicWriteService.ToggleFavorite(c.Context(), user.ID, tid); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// ToggleHide hides or unhides a topic.
// PUT /api/topic/:tid/hide
func (h *TopicHandler) ToggleHide(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("���效的话题 ID"))
	}

	if appErr := h.topicWriteService.ToggleHide(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.TopicHide), tid); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "操作成功")
}

// SetBestAnswer marks a reply as the best answer.
// PUT /api/topic/:tid/best-answer
func (h *TopicHandler) SetBestAnswer(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	tid, err := strconv.Atoi(c.Params("tid"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的话�� ID"))
	}

	var req dto.BestAnswerRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	if appErr := h.topicWriteService.SetBestAnswer(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.TopicSetBestAnswer), tid, req.ReplyID); appErr != nil {
		return response.Error(c, appErr)
	}

	return response.OKMessage(c, "已设置最佳回答")
}
