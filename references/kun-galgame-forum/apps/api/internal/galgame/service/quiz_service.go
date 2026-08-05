package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/trust/gate"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"gorm.io/gorm"
)

type QuizService struct {
	quizRepo      *repository.QuizRepository
	galgameClient *client.GalgameClient
	userClient    *userclient.Client
	check         *gate.CheckService
	scan          *gate.ScanService
	helpers       InteractionHelpers
}

func NewQuizService(
	quizRepo *repository.QuizRepository,
	galgameClient *client.GalgameClient,
	userClient *userclient.Client,
	check *gate.CheckService,
	scan *gate.ScanService,
) *QuizService {
	return &QuizService{
		quizRepo:      quizRepo,
		galgameClient: galgameClient,
		userClient:    userClient,
		check:         check,
		scan:          scan,
	}
}

// quizAuthoringModerationText composes the RAW text the trust gate sees for an
// authored quiz (create + edit): question + description + explanation + every
// free-text fragment inside the content JSONB.
func quizAuthoringModerationText(question, description, explanation, qtype string, content json.RawMessage) string {
	return gate.ComposeText(question, description, explanation, quizContentModerationText(qtype, content))
}

// quizCreateReward: flat authoring reward (被采纳), granted at create time and
// refunded on delete. Difficulty is ignored for now (kept in the signature so
// re-introducing a scaled reward stays a one-line change).
func quizCreateReward(_ int) int {
	return constants.QuizCreateReward
}

// quizCorrectReward: reward for a correct answer. Currently 0 (disabled).
func quizCorrectReward(_ int) int {
	return constants.QuizCorrectReward
}

// ──────────────────────────────────────────
// GetAllQuizzes — GET /galgame-quiz/all
// ──────────────────────────────────────────

func (s *QuizService) GetAllQuizzes(
	ctx context.Context,
	req *dto.QuizListRequest,
	isSFW bool,
	viewerID int,
) (*dto.QuizListPage, *errors.AppError) {
	sortOrder := req.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	rows, total := s.quizRepo.ListPaginated(model.QuizFilter{
		Category:   req.Category,
		Type:       req.Type,
		SortField:  req.SortField,
		SortOrder:  sortOrder,
		Difficulty: req.Difficulty,
		GalgameID:  req.GalgameID,
		UserID:     req.UserID,
		Page:       req.Page,
		Limit:      req.Limit,
	})
	return s.hydrateCards(ctx, rows, total, viewerID), nil
}

// GetMyAnswered — GET /galgame-quiz/mine/answered (self only).
func (s *QuizService) GetMyAnswered(
	ctx context.Context,
	userID, page, limit int,
) (*dto.QuizListPage, *errors.AppError) {
	rows, total := s.quizRepo.ListAnsweredByUser(userID, page, limit)
	return s.hydrateCards(ctx, rows, total, userID), nil
}

// hydrateCards resolves authors + stamps the viewer's own status, dropping rows
// whose author is banned. Cards show the category, not the linked games (which
// may be hidden until answered), so no galgame briefs are fetched here.
func (s *QuizService) hydrateCards(
	ctx context.Context,
	rows []model.GalgameQuizRow,
	total int64,
	viewerID int,
) *dto.QuizListPage {
	userIDs := make([]int, 0, len(rows))
	quizIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
		quizIDs = append(quizIDs, r.ID)
	}
	userMap := s.userClient.Hydrate(ctx, userIDs)
	viewerAnswers := s.quizRepo.FindViewerAnswers(quizIDs, viewerID)

	cards := make([]dto.QuizCard, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		card := quizRowToCard(r, u)
		va, answered := viewerAnswers[r.ID]
		card.MyStatus = quizViewerStatus(va, answered)
		cards = append(cards, card)
	}
	return &dto.QuizListPage{QuizData: cards, Total: total}
}

// quizViewerStatus maps the viewer's row to a status label for the list card.
func quizViewerStatus(va repository.QuizViewerAnswer, answered bool) string {
	switch {
	case !answered:
		return "unanswered"
	case va.Role == "author":
		return "author"
	case va.IsCorrect == nil:
		return "answered" // essay / ungraded
	case *va.IsCorrect:
		return "correct"
	default:
		return "incorrect"
	}
}

// ──────────────────────────────────────────
// GetQuizPlay — GET /galgame-quiz/:id
// ──────────────────────────────────────────

func (s *QuizService) GetQuizPlay(
	ctx context.Context,
	quizID, currentUserID int,
) (*dto.QuizPlay, *errors.AppError) {
	quiz, ok := s.quizRepo.FindByID(quizID)
	if !ok {
		return nil, errors.ErrNotFound("题目不存在")
	}
	// Hide a banned author's quiz even by direct link.
	author, _, _ := s.userClient.User(ctx, quiz.UserID)
	if !userclient.IsRenderable(author) {
		return nil, errors.ErrNotFound("题目不存在")
	}

	s.quizRepo.IncrementView(quizID)
	quiz.View++

	isAuthor := currentUserID != 0 && currentUserID == quiz.UserID

	// Reveal the answer key + explanation only once the viewer has a row
	// (answered, or the author's auto-row).
	var myAnswer *dto.QuizAnswerResult
	if currentUserID != 0 {
		if row, has := s.quizRepo.FindAnswer(quizID, currentUserID); has {
			myAnswer = &dto.QuizAnswerResult{
				Submitted:     row.Submitted,
				IsCorrect:     row.IsCorrect,
				Answer:        quiz.Content,
				Explanation:   quiz.Explanation,
				QualityRating: row.QualityRating,
			}
		}
	}

	// Linked games are revealed only if not hidden, or once the viewer has a
	// row (answered / is the author) — otherwise sent empty.
	galgames := []dto.QuizGalgameDetail{}
	if !quiz.HideGalgame || myAnswer != nil {
		galgames = s.galgamesDetailFor(ctx, s.quizRepo.FindQuizGalgameIDs(quizID))
	}

	play := &dto.QuizPlay{
		ID:              quiz.ID,
		User:            userBriefToDTO(author),
		Category:        quiz.Category,
		SpoilerLevel:    quiz.SpoilerLevel,
		Type:            quiz.Type,
		Difficulty:      quiz.Difficulty,
		Question:        quiz.Question,
		DescriptionHtml: markdown.Render(quiz.Description),
		Content:         stripQuizContent(quiz.Type, quiz.Content),
		QuizStats:       quizStats(quiz.View, quiz.AnswerCount, quiz.CorrectCount, quiz.FavoriteCount, quiz.QualitySum, quiz.QualityCount, quiz.CommentCount),
		Created:         quiz.CreatedAt.Format(time.RFC3339),
		Updated:         quiz.UpdatedAt.Format(time.RFC3339),
		HideGalgame:     quiz.HideGalgame,
		Galgames:        galgames,
		IsAuthor:        isAuthor,
		IsFavorited:     s.quizRepo.FindQuizFavorite(quizID, currentUserID),
		MyAnswer:        myAnswer,
	}
	return play, nil
}

// ──────────────────────────────────────────
// CreateQuiz — POST /galgame-quiz
// Anyone publishes immediately (no review gate in MVP). Grants the author the
// difficulty-scaled authoring reward, and seeds the author's roster row.
// ──────────────────────────────────────────

func (s *QuizService) CreateQuiz(
	ctx context.Context,
	userID int,
	req *dto.CreateQuizRequest,
) (*dto.CreatedQuiz, *errors.AppError) {
	if appErr := validateQuizContent(req.Type, req.Content); appErr != nil {
		return nil, appErr
	}

	// Synchronous word-list gate BEFORE the tx (trust wave 2): question +
	// description + explanation + the content's free text. deny blocks (nothing
	// persisted); hold publishes+logs; fail-open on error/timeout.
	moderationText := quizAuthoringModerationText(req.Question, req.Description, req.Explanation, req.Type, req.Content)
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return nil, gate.ErrContentBlocked()
	}

	spoiler := req.SpoilerLevel
	if spoiler == "" {
		spoiler = "none"
	}
	quiz := &model.GalgameQuiz{
		UserID:       userID,
		Category:     req.Category,
		SpoilerLevel: spoiler,
		Type:         req.Type,
		Difficulty:   req.Difficulty,
		Question:     req.Question,
		Description:  req.Description,
		Content:      req.Content,
		Explanation:  req.Explanation,
		HideGalgame:  req.HideGalgame,
		// Seed last-activity = creation time (bumped later on answer / edit).
		StatusUpdateTime: time.Now(),
	}
	reward := quizCreateReward(req.Difficulty)

	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.quizRepo.Create(tx, quiz); err != nil {
			return err
		}
		if err := s.quizRepo.SetQuizGalgames(tx, quiz.ID, req.GalgameIDs); err != nil {
			return err
		}
		// Author roster row: marks them a participant so they can't answer
		// their own question; carries no submitted answer / grade.
		if err := s.quizRepo.CreateAnswer(tx, &model.GalgameQuizAnswer{
			QuizID: quiz.ID, UserID: userID, Role: "author",
		}); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, userID, reward,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame_quiz", quiz.ID))
		return nil
	})
	if txErr != nil {
		return nil, errors.ErrInternal("创建题目失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindGalgameQuiz, "subject_id", quiz.ID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindGalgameQuiz, strconv.Itoa(quiz.ID), moderationText, int64(userID))

	author, _, _ := s.userClient.User(ctx, userID)
	return &dto.CreatedQuiz{
		ID:           quiz.ID,
		User:         userBriefToDTO(author),
		Category:     quiz.Category,
		SpoilerLevel: quiz.SpoilerLevel,
		Type:         quiz.Type,
		Difficulty:   quiz.Difficulty,
		Question:     quiz.Question,
		QuizStats:    quizStats(0, 0, 0, 0, 0, 0, 0),
		Created:      quiz.CreatedAt.Format(time.RFC3339),
		Updated:      quiz.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// ──────────────────────────────────────────
// AnswerQuiz — POST /galgame-quiz/:id/answer
// One attempt per user. Grades server-side; a correct answer grants the
// difficulty-scaled reward once. Reveals the answer key + explanation.
// ──────────────────────────────────────────

func (s *QuizService) AnswerQuiz(
	ctx context.Context,
	userID int,
	req *dto.AnswerQuizRequest,
) (*dto.QuizAnswerResult, *errors.AppError) {
	quiz, ok := s.quizRepo.FindByID(req.QuizID)
	if !ok {
		return nil, errors.ErrNotFound("题目不存在")
	}
	if existing, has := s.quizRepo.FindAnswer(req.QuizID, userID); has {
		if existing.Role == "author" {
			return nil, errors.ErrBadRequest("不能回答自己出的题目")
		}
		return nil, errors.ErrBadRequest("您已经回答过该题目了")
	}

	grade, appErr := gradeQuiz(quiz.Type, quiz.Content, req.Submitted)
	if appErr != nil {
		return nil, appErr
	}

	// Synchronous word-list gate BEFORE the tx — but ONLY when the submission
	// carries user free text (fill values / essay). Choice/judge answers are
	// indexes/booleans, so both check AND scan are skipped entirely for them.
	moderationText := quizAnswerModerationText(quiz.Type, req.Submitted)
	decision := gate.DecisionAllow
	var matched []string
	if moderationText != "" {
		authorID := int64(userID)
		decision, matched = s.check.Decision(ctx, moderationText, &authorID)
		if decision == gate.DecisionDeny {
			return nil, gate.ErrContentBlocked()
		}
	}

	correct := grade != nil && *grade
	reward := 0
	if correct {
		reward = quizCorrectReward(quiz.Difficulty)
	}

	// Notify the author of this answer + verdict (choice questions include the
	// chosen option text; essay is ungraded so it carries no verdict).
	notifyContent := quizAnswerSummary(quiz.Type, quiz.Content, req.Submitted)
	if grade != nil {
		if *grade {
			notifyContent += "，回答正确"
		} else {
			notifyContent += "，回答错误"
		}
	}

	row := &model.GalgameQuizAnswer{
		QuizID:    req.QuizID,
		UserID:    userID,
		Role:      "answerer",
		Submitted: req.Submitted,
		IsCorrect: grade,
		Rewarded:  reward > 0,
	}
	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.quizRepo.CreateAnswer(tx, row); err != nil {
			return err
		}
		if err := s.quizRepo.BumpAnswerStats(tx, req.QuizID, correct); err != nil {
			return err
		}
		s.helpers.CreateQuizAnswerMessage(tx, userID, quiz.UserID, notifyContent, req.QuizID)
		if reward > 0 {
			s.helpers.AdjustMoemoepoint(tx, userID, reward,
				moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame_quiz_answer", row.ID))
		}
		return nil
	})
	if txErr != nil {
		return nil, errors.ErrInternal("提交答案失败")
	}

	// Off-request shadow scan (only when the submission carried free text). The
	// subject id is the answer row id.
	if moderationText != "" {
		if decision == gate.DecisionHold {
			slog.Info("trust check hold", "subject_kind", gate.SubjectKindGalgameQuizAnswer, "subject_id", row.ID, "author_id", userID, "matched", matched)
		}
		s.scan.ScanBg(gate.SubjectKindGalgameQuizAnswer, strconv.Itoa(row.ID), moderationText, int64(userID))
	}

	return &dto.QuizAnswerResult{
		Submitted:   req.Submitted,
		IsCorrect:   grade,
		Answer:      quiz.Content,
		Explanation: quiz.Explanation,
		RewardDelta: reward,
	}, nil
}

// ──────────────────────────────────────────
// RateQuizQuality — PUT /galgame-quiz/:id/quality
// Only a genuine answerer may rate (not the author). Re-rating is allowed.
// ──────────────────────────────────────────

func (s *QuizService) RateQuizQuality(
	userID int,
	req *dto.RateQuizQualityRequest,
) (*dto.QuizQualityResult, *errors.AppError) {
	quiz, ok := s.quizRepo.FindByID(req.QuizID)
	if !ok {
		return nil, errors.ErrNotFound("题目不存在")
	}
	row, has := s.quizRepo.FindAnswer(req.QuizID, userID)
	if !has {
		return nil, errors.ErrForbidden("请先回答题目再评分")
	}
	if row.Role == "author" {
		return nil, errors.ErrForbidden("不能给自己出的题目评分")
	}

	sumDelta, countDelta := req.QualityRating, 1
	if row.QualityRating != nil {
		sumDelta, countDelta = req.QualityRating-*row.QualityRating, 0
	}

	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.quizRepo.SetAnswerQuality(tx, row.ID, req.QualityRating); err != nil {
			return err
		}
		return s.quizRepo.AdjustQuality(tx, req.QuizID, sumDelta, countDelta)
	})
	if txErr != nil {
		return nil, errors.ErrInternal("评分失败")
	}

	newSum := quiz.QualitySum + sumDelta
	newCount := quiz.QualityCount + countDelta
	return &dto.QuizQualityResult{
		QualityAverage: quizQualityAverage(newSum, newCount),
		QualityCount:   newCount,
		QualityRating:  req.QualityRating,
	}, nil
}

// ──────────────────────────────────────────
// DeleteQuiz — DELETE /galgame-quiz/:id
// Author or moderator. Refunds the author's create reward.
// ──────────────────────────────────────────

func (s *QuizService) DeleteQuiz(userID int, canModerate bool, quizID int) *errors.AppError {
	quiz, ok := s.quizRepo.FindByID(quizID)
	if !ok {
		return errors.ErrNotFound("题目不存在")
	}
	if quiz.UserID != userID && !canModerate {
		return errors.ErrForbidden("没有删除该题目的权限")
	}

	refund := -quizCreateReward(quiz.Difficulty)
	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.quizRepo.DeleteByID(tx, quizID); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, quiz.UserID, refund,
			moemoepoint.ReasonContentRemoved, moemoepoint.Ref("galgame_quiz", quizID))
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("删除题目失败")
	}
	return nil
}

// ──────────────────────────────────────────
// ToggleQuizFavorite — PUT /galgame-quiz/:id/favorite
// Mirrors topic favorites: toggle the (quiz, user) row, bump favorite_count,
// and grant the author ±1 moemoepoint (ReasonLiked). Self-favorites earn nothing.
// ──────────────────────────────────────────

func (s *QuizService) ToggleQuizFavorite(userID, quizID int) *errors.AppError {
	quiz, ok := s.quizRepo.FindByID(quizID)
	if !ok {
		return errors.ErrNotFound("题目不存在")
	}
	delta := 1
	if s.quizRepo.FindQuizFavorite(quizID, userID) {
		delta = -1
	}
	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if delta < 0 {
			if err := s.quizRepo.DeleteQuizFavorite(tx, quizID, userID); err != nil {
				return err
			}
		} else if err := s.quizRepo.CreateQuizFavorite(tx, quizID, userID); err != nil {
			return err
		}
		if err := s.quizRepo.AdjustQuizFavoriteCount(tx, quizID, delta); err != nil {
			return err
		}
		if userID != quiz.UserID {
			s.helpers.AdjustMoemoepoint(tx, quiz.UserID, delta,
				moemoepoint.ReasonLiked, moemoepoint.Ref("galgame_quiz", quizID))
		}
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

// ──────────────────────────────────────────
// UpdateQuiz — PUT /galgame-quiz/:id (author or moderator)
// Editing content does NOT re-grade existing answers (a known MVP limitation).
// ──────────────────────────────────────────

func (s *QuizService) UpdateQuiz(
	ctx context.Context,
	userID int, canModerate bool, req *dto.UpdateQuizRequest,
) (int, *errors.AppError) {
	quiz, ok := s.quizRepo.FindByID(req.QuizID)
	if !ok {
		return 0, errors.ErrNotFound("题目不存在")
	}
	if quiz.UserID != userID && !canModerate {
		return 0, errors.ErrForbidden("没有编辑该题目的权限")
	}
	if appErr := validateQuizContent(req.Type, req.Content); appErr != nil {
		return 0, appErr
	}

	// Synchronous word-list gate BEFORE the tx (deny blocks, hold publishes+logs,
	// fail-open). author_id is the content author (quiz.UserID).
	moderationText := quizAuthoringModerationText(req.Question, req.Description, req.Explanation, req.Type, req.Content)
	authorID := int64(quiz.UserID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return 0, gate.ErrContentBlocked()
	}

	spoiler := req.SpoilerLevel
	if spoiler == "" {
		spoiler = "none"
	}
	// Existing answers can be re-scored against the edited key only when the type
	// is unchanged — a type change is a different question, not a key correction.
	regradable := req.Type == quiz.Type
	fields := map[string]any{
		"category":      req.Category,
		"spoiler_level": spoiler,
		"type":          req.Type,
		"difficulty":    req.Difficulty,
		"question":      req.Question,
		"description":   req.Description,
		"content":       req.Content,
		"explanation":   req.Explanation,
		"hide_galgame":  req.HideGalgame,
		// An author edit counts as activity → bump the last-activity time.
		"status_update_time": gorm.Expr("now()"),
	}
	// Point the local model at the NEW key/difficulty so the regrade grades and
	// rewards against what was just saved (UpdateQuizFields writes from `fields`).
	quiz.Type = req.Type
	quiz.Content = req.Content
	quiz.Difficulty = req.Difficulty

	regraded := 0
	txErr := s.quizRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.quizRepo.UpdateQuizFields(tx, req.QuizID, fields); err != nil {
			return err
		}
		if err := s.quizRepo.SetQuizGalgames(tx, req.QuizID, req.GalgameIDs); err != nil {
			return err
		}
		if regradable {
			n, err := s.regradeAnswers(tx, quiz)
			if err != nil {
				return err
			}
			regraded = n
		}
		return nil
	})
	if txErr != nil {
		return 0, errors.ErrInternal("更新题目失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindGalgameQuiz, "subject_id", req.QuizID, "author_id", quiz.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindGalgameQuiz, strconv.Itoa(req.QuizID), moderationText, int64(quiz.UserID))
	return regraded, nil
}

// regradeAnswers re-scores every existing answer against the (possibly just
// corrected) answer key. ADDITIVE-ONLY: it only flips wrong→correct and back-pays
// the answer-correct reward that was missed — idempotent via the original
// galgame_quiz_answer:<id> ref — and NEVER flips correct→wrong nor claws
// moemoepoint back, so the author's key fix can't cost anyone points. Auto-graded
// types only (essay is never machine-graded). Returns how many were newly
// marked correct.
func (s *QuizService) regradeAnswers(tx *gorm.DB, quiz *model.GalgameQuiz) (int, error) {
	if quiz.Type == "essay" {
		return 0, nil
	}
	reward := quizCorrectReward(quiz.Difficulty)
	flipped := 0
	for _, a := range s.quizRepo.FindAnswerersForRegrade(quiz.ID) {
		grade, appErr := gradeQuiz(quiz.Type, quiz.Content, a.Submitted)
		if appErr != nil {
			continue // a submission that no longer fits the key shape — leave it
		}
		newCorrect := grade != nil && *grade
		wasCorrect := a.IsCorrect != nil && *a.IsCorrect
		if !newCorrect || wasCorrect {
			continue // additive-only: act on wrong→correct, nothing else
		}
		fields := map[string]any{"is_correct": true}
		if reward > 0 && !a.Rewarded {
			fields["rewarded"] = true
			s.helpers.AdjustMoemoepoint(tx, a.UserID, reward,
				moemoepoint.ReasonContentApproved,
				moemoepoint.Ref("galgame_quiz_answer", a.ID))
		}
		if err := s.quizRepo.UpdateAnswerFields(tx, a.ID, fields); err != nil {
			return 0, err
		}
		flipped++
	}
	if flipped > 0 {
		if err := s.quizRepo.AdjustCorrectCount(tx, quiz.ID, flipped); err != nil {
			return 0, err
		}
	}
	return flipped, nil
}

// ──────────────────────────────────────────
// GetQuizForEdit — GET /galgame-quiz/:id/edit (author or moderator)
// Returns the FULL quiz incl. the answer key so the edit form can pre-fill.
// ──────────────────────────────────────────

func (s *QuizService) GetQuizForEdit(
	ctx context.Context, userID int, canModerate bool, quizID int,
) (*dto.QuizEditData, *errors.AppError) {
	quiz, ok := s.quizRepo.FindByID(quizID)
	if !ok {
		return nil, errors.ErrNotFound("题目不存在")
	}
	if quiz.UserID != userID && !canModerate {
		return nil, errors.ErrForbidden("没有编辑该题目的权限")
	}
	galgameIDs := s.quizRepo.FindQuizGalgameIDs(quizID)
	return &dto.QuizEditData{
		ID:           quiz.ID,
		GalgameIDs:   galgameIDs,
		HideGalgame:  quiz.HideGalgame,
		Category:     quiz.Category,
		Type:         quiz.Type,
		Difficulty:   quiz.Difficulty,
		SpoilerLevel: quiz.SpoilerLevel,
		Question:     quiz.Question,
		Description:  quiz.Description,
		Content:      quiz.Content,
		Explanation:  quiz.Explanation,
		Galgames:     s.galgameBriefsFor(ctx, galgameIDs),
	}, nil
}

// ──────────────────────────────────────────
// GetMyFavorites returns the quiz ids the viewer has favorited — for the feed
// card's client-side favorite hydration.
func (s *QuizService) GetMyFavorites(userID int) []int {
	return s.quizRepo.FindFavoritedQuizIDs(userID)
}

// GetQuizAnswers — GET /galgame-quiz/:id/answers
// The genuine answerers + their grade, for the card 查看详情 panel.
// ──────────────────────────────────────────

const quizAnswerersLimit = 100

func (s *QuizService) GetQuizAnswers(
	ctx context.Context, quizID, viewerID int,
) []dto.QuizAnswererRecord {
	rows := s.quizRepo.FindQuizAnswerers(quizID, quizAnswerersLimit)
	if len(rows) == 0 {
		return []dto.QuizAnswererRecord{}
	}
	// A submitted answer reveals the correct answer, so surface it ONLY to a
	// viewer who has themselves engaged this quiz — answered it, or authored it
	// (both already know the answer). A non-answerer still sees who answered +
	// their grade, never the answers. Anonymous viewer (id 0) → no row → hidden.
	_, viewerEngaged := s.quizRepo.FindAnswer(quizID, viewerID)

	userIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
	}
	userMap := s.userClient.Hydrate(ctx, userIDs)
	records := make([]dto.QuizAnswererRecord, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		rec := dto.QuizAnswererRecord{
			User:      userBriefToDTO(u),
			IsCorrect: r.IsCorrect,
			Created:   r.Created,
		}
		if viewerEngaged {
			rec.Submitted = r.Submitted
		}
		records = append(records, rec)
	}
	return records
}

// ──────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────

// galgameBriefsFor returns id+name briefs (order-preserving) for the edit
// form's picker chips. No SFW gate — mirrors the rating/detail policy.
func (s *QuizService) galgameBriefsFor(ctx context.Context, ids []int) []dto.QuizGalgameBrief {
	out := []dto.QuizGalgameBrief{}
	if len(ids) == 0 {
		return out
	}
	m := s.fetchBriefs(ctx, ids)
	for _, id := range ids {
		b, ok := m[id]
		if !ok {
			continue
		}
		out = append(out, dto.QuizGalgameBrief{
			ID:           b.ID,
			ContentLimit: b.ContentLimit,
			Name: dto.KunLanguage{
				EnUs: b.NameEnUs, JaJp: b.NameJaJp,
				ZhCn: b.NameZhCn, ZhTw: b.NameZhTw,
			},
		})
	}
	return out
}

// galgamesDetailFor builds the richer linked-game panels (banner + 会社) for the
// answer page's 查看详情. isSFW=false — the detail page doesn't SFW-gate.
func (s *QuizService) galgamesDetailFor(ctx context.Context, ids []int) []dto.QuizGalgameDetail {
	out := []dto.QuizGalgameDetail{}
	if len(ids) == 0 {
		return out
	}
	m := s.fetchDetailBriefs(ctx, ids, false)
	for _, id := range ids {
		b, ok := m[id]
		if !ok {
			continue
		}
		banner := b.EffectiveBannerURL
		if banner == "" {
			banner = b.Banner
		}
		officials := b.Officials
		if officials == nil {
			officials = []string{}
		}
		out = append(out, dto.QuizGalgameDetail{
			ID: b.ID,
			Name: dto.KunLanguage{
				EnUs: b.NameEnUs, JaJp: b.NameJaJp,
				ZhCn: b.NameZhCn, ZhTw: b.NameZhTw,
			},
			ContentLimit:     b.ContentLimit,
			AgeLimit:         b.AgeLimit,
			OriginalLanguage: b.OriginalLanguage,
			Banner:           banner,
			BannerThumbhash:  b.EffectiveBannerThumbhash,
			Officials:        officials,
		})
	}
	return out
}

func (s *QuizService) fetchBriefs(ctx context.Context, galgameIDs []int) map[int]client.GalgameBrief {
	if len(galgameIDs) == 0 {
		return map[int]client.GalgameBrief{}
	}
	m, _ := s.galgameClient.GetBatch(ctx, galgameIDs)
	if m == nil {
		return map[int]client.GalgameBrief{}
	}
	return m
}

// ──────────────────────────────────────────
// SearchGalgameOptions — GET /galgame/search/picker
// Powers the 出题 modal's galgame picker: a name search enriched with each
// hit's banner + maker (会社) names. Soft-fails to an empty list.
// ──────────────────────────────────────────

// quizGalgameSearchLimit caps how many hits we enrich (one batch-detail call),
// keeping the picker snappy.
const quizGalgameSearchLimit = 12

func (s *QuizService) SearchGalgameOptions(
	ctx context.Context, keywords string, isSFW bool,
) []dto.QuizGalgameOption {
	empty := []dto.QuizGalgameOption{}
	// The catalog product search is the SAME ranked, typo-tolerant search the
	// /search page uses. Only the hit ids are read here — the picker hydrates
	// them through the batch-detail call below, which is where the banner and
	// maker names come from.
	q := url.Values{
		"q":     {keywords},
		"limit": {strconv.Itoa(quizGalgameSearchLimit)},
		"sort":  {"relevance"},
		// Only works kungal can actually address are pickable: an unclaimed
		// catalog work has no gid, and a quiz option must point at a galgame the
		// forum can render. `claimed` alone still admits an entry the wiki
		// claimed but never published, so a quiz could be authored against a
		// game no player can open — the authoring-side face of the §37 leak.
		"claimed":     {"true"},
		"claim_state": {"live"},
	}
	client.ApplyWorksGate(q, isSFW)
	res, appErr := s.galgameClient.CatalogWorksSearch(ctx, q)
	if appErr != nil {
		return empty
	}
	ids := make([]int, 0, quizGalgameSearchLimit)
	for i := range res.Items {
		if !client.CatalogItemRenderable(&res.Items[i]) {
			continue
		}
		if gid := client.CatalogItemGID(&res.Items[i]); gid > 0 {
			ids = append(ids, gid)
		}
		if len(ids) >= quizGalgameSearchLimit {
			break
		}
	}
	if len(ids) == 0 {
		return empty
	}

	briefs := s.fetchDetailBriefs(ctx, ids, isSFW)
	options := make([]dto.QuizGalgameOption, 0, len(ids))
	for _, id := range ids {
		b, ok := briefs[id]
		if !ok {
			continue // SFW-filtered or missing
		}
		banner := b.EffectiveBannerURL
		if banner == "" {
			banner = b.Banner
		}
		officials := b.Officials
		if officials == nil {
			officials = []string{}
		}
		options = append(options, dto.QuizGalgameOption{
			ID: b.ID,
			Name: dto.KunLanguage{
				EnUs: b.NameEnUs, JaJp: b.NameJaJp,
				ZhCn: b.NameZhCn, ZhTw: b.NameZhTw,
			},
			Banner:          banner,
			BannerThumbhash: b.EffectiveBannerThumbhash,
			Officials:       officials,
		})
	}
	return options
}

func (s *QuizService) fetchDetailBriefs(ctx context.Context, ids []int, isSFW bool) map[int]client.GalgameDetailBrief {
	if len(ids) == 0 {
		return map[int]client.GalgameDetailBrief{}
	}
	m, _ := s.galgameClient.GetBatchDetailPublic(ctx, ids, isSFW)
	if m == nil {
		return map[int]client.GalgameDetailBrief{}
	}
	return m
}
