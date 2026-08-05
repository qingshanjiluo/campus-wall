package service

// Read/write gating for comment areas that sit on top of concealed content.
//
// The quiz area is the only one with something to protect: a quiz may hide its
// linked galgames until the viewer answers (hide_galgame, migration 047) and may
// declare a spoiler level (migration 046). A comment area on such a quiz is a
// straight answer-leak channel — one "答案是 XX" top post retires the question —
// so comments are withheld from a viewer who has neither answered nor authored
// it.
//
// This MUST be enforced here rather than in the frontend: the comment list is a
// public GET, so a frontend-only placeholder would still serve the answer to
// anyone who opened the network tab. The decision is also returned to the client
// (CommunityCommentPage.Locked) so the placeholder copy is driven by the server's
// ruling instead of the rule being re-implemented — and drifting — in the UI.
//
// Quizzes that conceal nothing (no hidden galgames, spoiler level none/empty) are
// never gated: withholding discussion there would be friction with nothing to
// protect.

import "context"

// quizGateLocked is the PURE gate decision. A quiz conceals something when it
// hides its linked galgames or carries a non-trivial spoiler level; an empty
// spoiler_level (pre-migration-046 rows) counts as none. The author always sees
// their own quiz's discussion, and so does anyone who has answered it.
func quizGateLocked(hideGalgame bool, spoilerLevel string, isAuthor, hasAnswered bool) bool {
	conceals := hideGalgame || (spoilerLevel != "" && spoilerLevel != "none")
	if !conceals {
		return false
	}
	return !isAuthor && !hasAnswered
}

// commentAreaLocked reports whether this viewer must be kept out of the area's
// comments. Only the quiz area gates; every other source is always open. An
// anonymous viewer (viewerID 0) can never be the author and has no answer row, so
// a concealing quiz is locked for them.
func (s *ResourceCommentService) commentAreaLocked(_ context.Context, src CommentSource, resourceID, viewerID int) bool {
	if src.key != sourceQuiz.key {
		return false
	}

	var row struct {
		UserID       int    `gorm:"column:user_id"`
		HideGalgame  bool   `gorm:"column:hide_galgame"`
		SpoilerLevel string `gorm:"column:spoiler_level"`
	}
	res := s.db.Table("galgame_quiz").
		Select("user_id, hide_galgame, spoiler_level").
		Where("id = ?", resourceID).Limit(1).Find(&row)
	if res.Error != nil {
		// Fail CLOSED: if we cannot tell whether the quiz conceals anything, do not
		// serve its comments. A missing quiz (RowsAffected 0) is also locked; the
		// create path 404s it separately via resolveCreateCtx.
		return true
	}
	if res.RowsAffected == 0 {
		return true
	}

	isAuthor := viewerID != 0 && viewerID == row.UserID
	return quizGateLocked(row.HideGalgame, row.SpoilerLevel, isAuthor, s.hasAnsweredQuiz(resourceID, viewerID))
}

// hasAnsweredQuiz reports whether the viewer has an answer row for this quiz.
// Anonymous (0) never has one. A DB error reads as "not answered" — the caller
// only widens the gate with a true, never narrows it, so erring this way keeps the
// spoiler protected.
func (s *ResourceCommentService) hasAnsweredQuiz(quizID, viewerID int) bool {
	if viewerID == 0 {
		return false
	}
	var count int64
	if err := s.db.Table("galgame_quiz_answer").
		Where("quiz_id = ? AND user_id = ?", quizID, viewerID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}
