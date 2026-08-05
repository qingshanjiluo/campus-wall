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

type QuizHandler struct {
	quizService *service.QuizService
}

func NewQuizHandler(quizService *service.QuizService) *QuizHandler {
	return &QuizHandler{quizService: quizService}
}

// GetAllQuizzes — GET /api/galgame-quiz/all
//
// SFW-default: anonymous + cookie-less requests get only quizzes whose linked
// galgame is content_limit=sfw (general-trivia quizzes are always shown).
func (h *QuizHandler) GetAllQuizzes(c fiber.Ctx) error {
	var req dto.QuizListRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.quizService.GetAllQuizzes(
		c.Context(), &req, utils.IsSFW(c), optionalUID(c),
	)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetMyAnswered — GET /api/galgame-quiz/mine/answered (self only)
func (h *QuizHandler) GetMyAnswered(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.QuizListRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	page, appErr := h.quizService.GetMyAnswered(c.Context(), user.ID, req.Page, req.Limit)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, page)
}

// GetMyFavorites — GET /api/galgame-quiz/mine/favorites (self)
func (h *QuizHandler) GetMyFavorites(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, fiber.Map{"favorited": h.quizService.GetMyFavorites(user.ID)})
}

// SearchGalgames — GET /api/galgame/search/picker
// Name search for the 出题 picker, enriched with banner + 会社.
func (h *QuizHandler) SearchGalgames(c fiber.Ctx) error {
	var req dto.QuizGalgameSearchRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	options := h.quizService.SearchGalgameOptions(
		c.Context(), req.Keywords, utils.IsSFW(c),
	)
	return response.OK(c, options)
}

// GetQuizPlay — GET /api/galgame-quiz/:id
func (h *QuizHandler) GetQuizPlay(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的题目 ID"))
	}
	detail, appErr := h.quizService.GetQuizPlay(c.Context(), id, optionalUID(c))
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, detail)
}

// CreateQuiz — POST /api/galgame-quiz
func (h *QuizHandler) CreateQuiz(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.CreateQuizRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	created, appErr := h.quizService.CreateQuiz(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, created)
}

// AnswerQuiz — POST /api/galgame-quiz/:id/answer
func (h *QuizHandler) AnswerQuiz(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.AnswerQuizRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	result, appErr := h.quizService.AnswerQuiz(c.Context(), user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// RateQuizQuality — PUT /api/galgame-quiz/:id/quality
func (h *QuizHandler) RateQuizQuality(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.RateQuizQualityRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	result, appErr := h.quizService.RateQuizQuality(user.ID, &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, result)
}

// DeleteQuiz — DELETE /api/galgame-quiz/:id
func (h *QuizHandler) DeleteQuiz(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.DeleteQuizRequest
	if appErr := utils.ParseQueryAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	if appErr := h.quizService.DeleteQuiz(user.ID, perm.CanUser(user.ID, user.Roles, perm.QuizDeleteAny), req.QuizID); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "题目已删除")
}

// ToggleQuizFavorite — PUT /api/galgame-quiz/:id/favorite
func (h *QuizHandler) ToggleQuizFavorite(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的题目 ID"))
	}
	if appErr := h.quizService.ToggleQuizFavorite(user.ID, id); appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OKMessage(c, "操作成功")
}

// UpdateQuiz — PUT /api/galgame-quiz/:id (author or moderator)
func (h *QuizHandler) UpdateQuiz(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateQuizRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}
	regraded, appErr := h.quizService.UpdateQuiz(c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.QuizEditAny), &req)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	// `regraded` = answers an answer-key correction just flipped wrong→correct
	// (0 normally). The FE composes the toast from it (see Form.vue).
	return response.OK(c, fiber.Map{"regraded": regraded})
}

// GetQuizForEdit — GET /api/galgame-quiz/:id/edit (author or moderator)
func (h *QuizHandler) GetQuizForEdit(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的题目 ID"))
	}
	data, appErr := h.quizService.GetQuizForEdit(
		c.Context(), user.ID, perm.CanUser(user.ID, user.Roles, perm.QuizEditAny), id,
	)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	return response.OK(c, data)
}

// GetQuizAnswers — GET /api/galgame-quiz/:id/answers
func (h *QuizHandler) GetQuizAnswers(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return response.Error(c, errors.ErrBadRequest("无效的题目 ID"))
	}
	// OptionalAuth-populated viewer (nil when anonymous). The service reveals
	// each answerer's submitted answer only to a viewer who has engaged the quiz.
	viewerID := 0
	if u := middleware.GetUser(c); u != nil {
		viewerID = u.ID
	}
	return response.OK(c, h.quizService.GetQuizAnswers(c.Context(), id, viewerID))
}
