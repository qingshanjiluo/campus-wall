package dto

import "encoding/json"

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

// QuizListRequest is the query for GET /galgame-quiz/all. Difficulty/GalgameID/
// UserID default to 0 = "all".
type QuizListRequest struct {
	Page       int    `query:"page" validate:"min=1"`
	Limit      int    `query:"limit" validate:"min=1,max=50"`
	SortField  string `query:"sort_field"`
	SortOrder  string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
	Category   string `query:"category"`
	Type       string `query:"type"`
	Difficulty int    `query:"difficulty" validate:"omitempty,min=1,max=10"`
	GalgameID  int    `query:"galgame_id"`
	UserID     int    `query:"user_id"`
}

// CreateQuizRequest is the body of POST /galgame-quiz. `content` is the
// type-specific payload (options + answer key); it is shape-validated in the
// service against `type` before insert.
type CreateQuizRequest struct {
	GalgameIDs   []int           `json:"galgame_ids" validate:"omitempty,dive,min=1"`
	HideGalgame  bool            `json:"hide_galgame"`
	Category     string          `json:"category" validate:"required,oneof=plot character system music voice company trivia other"`
	Type         string          `json:"type" validate:"required,oneof=single multiple judge fill essay"`
	Difficulty   int             `json:"difficulty" validate:"required,min=1,max=10"`
	SpoilerLevel string          `json:"spoiler_level" validate:"omitempty,oneof=none portion serious"`
	Question     string          `json:"question" validate:"required,min=1,max=200"`
	Description  string          `json:"description" validate:"max=20000"`
	Content      json.RawMessage `json:"content" validate:"required"`
	Explanation  string          `json:"explanation" validate:"max=2000"`
}

// UpdateQuizRequest is the body of PUT /galgame-quiz/:id (author or moderator).
type UpdateQuizRequest struct {
	QuizID       int             `json:"quiz_id" validate:"required,min=1"`
	GalgameIDs   []int           `json:"galgame_ids" validate:"omitempty,dive,min=1"`
	HideGalgame  bool            `json:"hide_galgame"`
	Category     string          `json:"category" validate:"required,oneof=plot character system music voice company trivia other"`
	Type         string          `json:"type" validate:"required,oneof=single multiple judge fill essay"`
	Difficulty   int             `json:"difficulty" validate:"required,min=1,max=10"`
	SpoilerLevel string          `json:"spoiler_level" validate:"omitempty,oneof=none portion serious"`
	Question     string          `json:"question" validate:"required,min=1,max=200"`
	Description  string          `json:"description" validate:"max=20000"`
	Content      json.RawMessage `json:"content" validate:"required"`
	Explanation  string          `json:"explanation" validate:"max=2000"`
}

// QuizEditData is the response of GET /galgame-quiz/:id/edit — the FULL quiz
// (incl. the answer key in `content`), returned only to the author or a
// moderator so the edit form can be pre-filled.
type QuizEditData struct {
	ID           int             `json:"id"`
	GalgameIDs   []int           `json:"galgame_ids"`
	HideGalgame  bool            `json:"hide_galgame"`
	Category     string          `json:"category"`
	Type         string          `json:"type"`
	Difficulty   int             `json:"difficulty"`
	SpoilerLevel string          `json:"spoiler_level"`
	Question     string          `json:"question"`
	Description  string          `json:"description"`
	Content      json.RawMessage `json:"content"`
	Explanation  string          `json:"explanation"`
	// Briefs (id + name) of the linked games, to pre-fill the picker chips.
	Galgames []QuizGalgameBrief `json:"galgames"`
}

// QuizAnswererRecord is one answerer's outcome, for the card's 查看详情 panel.
type QuizAnswererRecord struct {
	User UserBrief `json:"user"`
	// Submitted is the answerer's own submission — surfaced ONLY to a viewer who
	// has themselves answered (or authored) this quiz (see GetQuizAnswers), since
	// it would otherwise leak the correct answer. Omitted for non-engaged viewers.
	Submitted json.RawMessage `json:"submitted,omitempty"`
	IsCorrect *bool           `json:"is_correct"`
	Created   string          `json:"created"`
}

// AnswerQuizRequest is the body of POST /galgame-quiz/:id/answer. `submitted`
// is the type-specific answer, graded server-side.
type AnswerQuizRequest struct {
	QuizID    int             `json:"quiz_id" validate:"required,min=1"`
	Submitted json.RawMessage `json:"submitted" validate:"required"`
}

// RateQuizQualityRequest is the body of PUT /galgame-quiz/:id/quality.
type RateQuizQualityRequest struct {
	QuizID        int `json:"quiz_id" validate:"required,min=1"`
	QualityRating int `json:"quality_rating" validate:"required,min=1,max=10"`
}

// DeleteQuizRequest is the query for DELETE /galgame-quiz/:id.
type DeleteQuizRequest struct {
	QuizID int `query:"quiz_id" validate:"required,min=1"`
}

// ──────────────────────────────────────────
// Responses — shared
// ──────────────────────────────────────────

// QuizGalgameBrief is the lightweight linked-game info on a quiz. Nil (a
// literal JSON `null`) when the quiz is general trivia.
type QuizGalgameBrief struct {
	ID           int         `json:"id"`
	ContentLimit string      `json:"content_limit"`
	Name         KunLanguage `json:"name"`
}

// QuizGalgameDetail is the richer linked-game panel on the answer page's
// 查看详情 (banner + 会社 + age), beyond the lightweight card brief.
type QuizGalgameDetail struct {
	ID               int         `json:"id"`
	Name             KunLanguage `json:"name"`
	ContentLimit     string      `json:"content_limit"`
	AgeLimit         string      `json:"age_limit"`
	OriginalLanguage string      `json:"original_language"`
	Banner           string      `json:"banner"`
	BannerThumbhash  string      `json:"banner_thumbhash,omitempty"`
	Officials        []string    `json:"officials"`
}

// QuizStats is the aggregate answer/quality block embedded in card + play.
type QuizStats struct {
	View           int     `json:"view"`
	AnswerCount    int     `json:"answer_count"`
	CorrectCount   int     `json:"correct_count"`
	FavoriteCount  int     `json:"favorite_count"`
	QualityAverage float64 `json:"quality_average"`
	QualityCount   int     `json:"quality_count"`
	// CommentCount is the quiz discussion counter (migration 065), maintained ±1
	// by the community comment BFF. A tolerated display counter.
	CommentCount int `json:"comment_count"`
}

// ──────────────────────────────────────────
// Responses — list
// ──────────────────────────────────────────

// QuizCard is a single entry in the quiz list response. It never carries the
// `content` payload — cards only show the prompt + meta + stats.
type QuizCard struct {
	ID           int       `json:"id"`
	User         UserBrief `json:"user"`
	Category     string    `json:"category"`
	Type         string    `json:"type"`
	Difficulty   int       `json:"difficulty"`
	SpoilerLevel string    `json:"spoiler_level"`
	Question     string    `json:"question"`
	QuizStats              // embedded view/answer/correct/quality
	Created      string    `json:"created"`
	Updated      string    `json:"updated"`
	// Last-activity time (bumped on answer / author edit) — the list's 最近活跃 sort.
	StatusUpdateTime string `json:"status_update_time"`
	// The viewer's own status on this quiz:
	// author | correct | incorrect | answered | unanswered.
	MyStatus string `json:"my_status"`
}

type QuizListPage struct {
	QuizData []QuizCard `json:"quiz_data"`
	Total    int64      `json:"total"`
}

// ──────────────────────────────────────────
// Responses — detail / play
// ──────────────────────────────────────────

// QuizPlay is the answer view (GET /galgame-quiz/:id). `Content` is the
// STRIPPED public payload — no correct-answer key. `MyAnswer` is non-nil once
// the viewer has answered (or is the author), and only then reveals the full
// answer key + explanation.
type QuizPlay struct {
	ID              int             `json:"id"`
	User            UserBrief       `json:"user"`
	Category        string          `json:"category"`
	Type            string          `json:"type"`
	Difficulty      int             `json:"difficulty"`
	SpoilerLevel    string          `json:"spoiler_level"`
	Question        string          `json:"question"`
	DescriptionHtml string          `json:"description_html"`
	Content         json.RawMessage `json:"content"`
	QuizStats                       // embedded view/answer/correct/quality
	Created         string          `json:"created"`
	Updated         string          `json:"updated"`
	HideGalgame     bool            `json:"hide_galgame"`
	// Empty when hide_galgame && the viewer hasn't answered yet.
	Galgames    []QuizGalgameDetail `json:"galgames"`
	IsAuthor    bool                `json:"is_author"`
	IsFavorited bool                `json:"is_favorited"`
	MyAnswer    *QuizAnswerResult   `json:"my_answer"`
}

// QuizAnswerResult reveals the graded outcome + the full answer key. It is the
// response of POST /galgame-quiz/:id/answer and the `my_answer` block of
// QuizPlay once the viewer has participated.
type QuizAnswerResult struct {
	Submitted     json.RawMessage `json:"submitted"`      // the viewer's answer (null for the author row)
	IsCorrect     *bool           `json:"is_correct"`     // null for essay / author
	Answer        json.RawMessage `json:"answer"`         // full content payload, key revealed
	Explanation   string          `json:"explanation"`    // author's解析, shown post-answer
	QualityRating *int            `json:"quality_rating"` // the viewer's quality vote, null if unrated
	RewardDelta   int             `json:"reward_delta"`   // moemoepoint just granted (answer POST only; 0 on GET)
}

// CreatedQuiz is the response of POST /galgame-quiz — the freshly authored card.
type CreatedQuiz struct {
	ID           int       `json:"id"`
	User         UserBrief `json:"user"`
	Category     string    `json:"category"`
	Type         string    `json:"type"`
	Difficulty   int       `json:"difficulty"`
	SpoilerLevel string    `json:"spoiler_level"`
	Question     string    `json:"question"`
	QuizStats              // zeroed stats on a fresh quiz
	Created      string    `json:"created"`
	Updated      string    `json:"updated"`
}

// QuizQualityResult is the response of PUT /galgame-quiz/:id/quality.
type QuizQualityResult struct {
	QualityAverage float64 `json:"quality_average"`
	QualityCount   int     `json:"quality_count"`
	QualityRating  int     `json:"quality_rating"`
}

// QuizGalgameSearchRequest is the query for GET /galgame/search/picker.
type QuizGalgameSearchRequest struct {
	Keywords string `query:"keywords" validate:"required,min=1"`
}

// QuizGalgameOption is a galgame candidate for the 出题 picker — name + banner
// thumbnail + 会社 (maker names), so the picker can show a rich result row.
type QuizGalgameOption struct {
	ID              int         `json:"id"`
	Name            KunLanguage `json:"name"`
	Banner          string      `json:"banner"`
	BannerThumbhash string      `json:"banner_thumbhash,omitempty"`
	Officials       []string    `json:"officials"`
}
