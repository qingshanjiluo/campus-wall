package model

import (
	"encoding/json"
	"time"
)

// GalgameQuiz is the writable model for the galgame_quiz table.
//
// `Content` is JSONB and holds the type-specific payload INCLUDING the correct
// answer key — it is never serialized verbatim to a client that has not
// answered (the service strips it via stripQuizContent). Linked galgames live
// in the galgame_quiz_galgame join table (many-to-many), not on this row.
// `Description` is markdown (rendered server-side); `HideGalgame` hides the
// linked games until the viewer answers.
type GalgameQuiz struct {
	ID           int             `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int             `gorm:"column:user_id;not null" json:"user_id"`
	Category     string          `gorm:"type:varchar(16)" json:"category"`
	SpoilerLevel string          `gorm:"column:spoiler_level;type:varchar(16)" json:"spoiler_level"`
	Type         string          `gorm:"type:varchar(16)" json:"type"`
	Difficulty   int             `gorm:"type:smallint" json:"difficulty"`
	Question     string          `gorm:"type:text" json:"question"`
	Description  string          `gorm:"type:text" json:"description"`
	Content      json.RawMessage `gorm:"type:jsonb" json:"content"`
	Explanation  string          `gorm:"type:text" json:"explanation"`
	HideGalgame  bool            `gorm:"column:hide_galgame;default:false" json:"hide_galgame"`
	View         int             `gorm:"default:0" json:"view"`

	AnswerCount   int `gorm:"column:answer_count;default:0" json:"answer_count"`
	CorrectCount  int `gorm:"column:correct_count;default:0" json:"correct_count"`
	FavoriteCount int `gorm:"column:favorite_count;default:0" json:"favorite_count"`
	// CommentCount is the discussion counter (migration 065), maintained ±1 by the
	// community comment BFF — a tolerated display counter, not a source of truth.
	CommentCount int `gorm:"column:comment_count;default:0" json:"comment_count"`
	QualitySum   int `gorm:"column:quality_sum;default:0" json:"quality_sum"`
	QualityCount int `gorm:"column:quality_count;default:0" json:"quality_count"`

	// StatusUpdateTime is the last-activity time (mirrors topic): defaults to
	// created, bumped when a user answers or the author edits — drives the 题库
	// list's "最近活跃" sort.
	StatusUpdateTime time.Time `gorm:"column:status_update_time;default:now()" json:"status_update_time"`
	CreatedAt        time.Time `gorm:"column:created" json:"created"`
	UpdatedAt        time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameQuiz) TableName() string { return "galgame_quiz" }

// quizBumpWindowDays is how long after a quiz's publish time (`created`) an
// interaction still floats it up the last-activity list.
//
// Necro-bump prevention (product rule): once a quiz is older than this window,
// nothing resets status_update_time anymore EXCEPT a re-edit of the quiz itself
// — answering it no longer resurfaces it. Within the window every answer bumps
// as before, and the edit path bumps regardless of age.
const quizBumpWindowDays = 3

// QuizBumpCutoff returns the publish-time boundary at instant `now`: a quiz whose
// `created` is at or before this instant has aged out of its bump window. The
// answer path gates its status_update_time bump on `created > QuizBumpCutoff(now)`.
func QuizBumpCutoff(now time.Time) time.Time {
	return now.AddDate(0, 0, -quizBumpWindowDays)
}

// GalgameQuizAnswer is one row per (quiz, user). It is at once the participant
// roster and the quality-vote store — see the DDL header in migration 045.
type GalgameQuizAnswer struct {
	ID        int             `gorm:"primaryKey;autoIncrement" json:"id"`
	QuizID    int             `gorm:"column:quiz_id;not null" json:"quiz_id"`
	UserID    int             `gorm:"column:user_id;not null" json:"user_id"`
	Role      string          `gorm:"type:varchar(16);default:answerer" json:"role"`
	Submitted json.RawMessage `gorm:"type:jsonb" json:"submitted"`
	IsCorrect *bool           `gorm:"column:is_correct" json:"is_correct"`
	Rewarded  bool            `gorm:"column:rewarded;default:false" json:"rewarded"`
	// QualityRating is the user's 1-10 quality vote (NULL until they rate).
	QualityRating *int `gorm:"column:quality_rating" json:"quality_rating"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (GalgameQuizAnswer) TableName() string { return "galgame_quiz_answer" }

// GalgameQuizGalgame is a row of the quiz↔galgame join table (many-to-many).
type GalgameQuizGalgame struct {
	QuizID    int `gorm:"column:quiz_id;primaryKey"`
	GalgameID int `gorm:"column:galgame_id;primaryKey"`
}

func (GalgameQuizGalgame) TableName() string { return "galgame_quiz_galgame" }

// GalgameQuizFavorite is one row per (quiz, user) who favorited it. `created`
// is left to the DB default (no struct field → GORM won't insert a zero time).
type GalgameQuizFavorite struct {
	QuizID int `gorm:"column:quiz_id;primaryKey"`
	UserID int `gorm:"column:user_id;primaryKey"`
}

func (GalgameQuizFavorite) TableName() string { return "galgame_quiz_favorite" }

// GalgameQuizRow is a flat projection of galgame_quiz for list reads (no
// `content` — cards never expose the payload). Identity + linked-game briefs
// are hydrated at the service layer.
type GalgameQuizRow struct {
	ID               int    `gorm:"column:id"`
	UserID           int    `gorm:"column:user_id"`
	Category         string `gorm:"column:category"`
	SpoilerLevel     string `gorm:"column:spoiler_level"`
	Type             string `gorm:"column:type"`
	Difficulty       int    `gorm:"column:difficulty"`
	Question         string `gorm:"column:question"`
	View             int    `gorm:"column:view"`
	AnswerCount      int    `gorm:"column:answer_count"`
	CorrectCount     int    `gorm:"column:correct_count"`
	FavoriteCount    int    `gorm:"column:favorite_count"`
	CommentCount     int    `gorm:"column:comment_count"`
	QualitySum       int    `gorm:"column:quality_sum"`
	QualityCount     int    `gorm:"column:quality_count"`
	StatusUpdateTime string `gorm:"column:status_update_time"`
	Created          string `gorm:"column:created"`
	Updated          string `gorm:"column:updated"`
}

// GalgameQuizAnswererRow is a flat projection of an answerer's outcome, for the
// card 查看详情 panel. Identity is hydrated at the service layer.
type GalgameQuizAnswererRow struct {
	UserID    int             `gorm:"column:user_id"`
	Submitted json.RawMessage `gorm:"column:submitted"`
	IsCorrect *bool           `gorm:"column:is_correct"`
	Created   string          `gorm:"column:created"`
}

// QuizFilter carries the list-query filters to the repository. Zero-valued
// scalar filters (Difficulty/GalgameID/UserID == 0) mean "no filter".
type QuizFilter struct {
	Category   string
	Type       string
	SortField  string
	SortOrder  string
	Difficulty int
	GalgameID  int
	UserID     int
	Page       int
	Limit      int
}
