package gate

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/pkg/trustclient"
)

// Registered trust subject kinds for the forum write surface (pre-registered on
// kungal's site — onboarding §4). Every edit is one fresh scan; the receiving
// face is idempotent.
const (
	SubjectKindTopic = "forum_topic"
	SubjectKindReply = "forum_reply"

	// Wave 2 — the remaining forum user-text write surfaces. Topic comments
	// REUSE the already-registered forum_comment kind (shared with the report +
	// enforcement paths and the FE report button); the rest are new kinds that
	// infra must register before scan stops 422-ing them (harmless warn-log until
	// then; the global flags are off in prod anyway).
	SubjectKindTopicComment      = "forum_comment"
	SubjectKindTopicPoll         = "forum_topic_poll"
	SubjectKindGalgameRating     = "galgame_rating"
	SubjectKindGalgameResource   = "galgame_resource"
	SubjectKindGalgameCollection = "galgame_collection"
	SubjectKindGalgameQuiz       = "galgame_quiz"
	SubjectKindGalgameQuizAnswer = "galgame_quiz_answer"
	SubjectKindToolset           = "galgame_toolset"
	SubjectKindToolsetResource   = "galgame_toolset_resource"
)

// scanTimeout bounds a single off-request scan call so stray goroutines can't
// pile up against a wedged trust service. Generous vs. the check gate — scanning
// is off the request path, a miss is harmless.
const scanTimeout = 5 * time.Second

// Scanner is the trust scan-face client, abstracted so tests fake it.
// *trustclient.Client satisfies it directly.
type Scanner interface {
	Scan(ctx context.Context, req trustclient.ScanRequest) (*trustclient.ScanResult, error)
}

// ScanService feeds a published topic/reply's RAW text into the trust shadow-
// scan pipeline AFTER commit. Pure best-effort: no outbox, no retry — scan
// events are shadow-calibration corpus, an edit self-heals a miss, and the
// receiving face is repeatable. A nil Scanner (or nil *ScanService) = scanning
// off: every path is a no-op.
type ScanService struct {
	sc Scanner
}

// NewScanService builds the service. sc nil = scanning off.
func NewScanService(sc Scanner) *ScanService {
	return &ScanService{sc: sc}
}

// Enabled reports whether scanning is wired.
func (s *ScanService) Enabled() bool { return s != nil && s.sc != nil }

// ScanBg best-effort scans one subject off the request path (fire-and-forget
// goroutine, logs on failure). Callers invoke it AFTER their write transaction
// commits, so a slow/down trust service never blocks or delays the create/edit
// response. author_id is optional (<=0 omits it).
func (s *ScanService) ScanBg(subjectKind, subjectID, text string, authorID int64) {
	if !s.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
		defer cancel()
		req := trustclient.ScanRequest{SubjectKind: subjectKind, SubjectID: subjectID, Text: text}
		if authorID > 0 {
			req.AuthorID = &authorID
		}
		if _, err := s.sc.Scan(ctx, req); err != nil {
			slog.Warn("trust scan", "subject_kind", subjectKind, "subject_id", subjectID, "err", err)
		}
	}()
}
