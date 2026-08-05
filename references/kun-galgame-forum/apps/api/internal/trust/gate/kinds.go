package gate

import "kun-galgame-api/pkg/trustclient"

// Report / enforcement-only subject kinds — content the forum never shadow-scans
// (so they are NOT in scan.go), but which the trust registry must still know
// about so /trust/reports and enforcement callbacks resolve instead of 422-ing:
//
//   - galgame_comment: ENFORCEMENT-only. Galgame-comment reports route to the
//     community moderation primitive (not /trust/reports); the trust enforcement
//     callback resolves the legacy id and tombstones the migrated community post
//     (see app.go's enforce registry, which keys this same literal).
//   - galgame / user: REPORT-only. The report button targets a galgame detail
//     page / a user profile, so both must be registered or the report 422s;
//     their LOCAL enforcement no-ops (galgame moderation is galgame-side, user bans
//     are IdP-side).
const (
	SubjectKindGalgameComment = "galgame_comment"
	SubjectKindGalgame        = "galgame"
	SubjectKindUser           = "user"
)

// CanonicalSubjectKinds is the SINGLE source of the forum's COMPLETE subject-
// kind universe. It is the union (deduped) of every kind the forum:
//   - sends to /trust/scan  — the SubjectKind* scan constants (scan.go),
//   - submits to /trust/reports — the FE report buttons (forum_topic/reply/
//     comment + galgame + user), and
//   - receives enforcement callbacks for — app.go's enforce registry
//     (forum_topic/reply/comment + galgame_comment).
//
// It is declared to the trust registry via a best-effort startup ensure call
// (trustclient.EnsureSubjectKinds in app.go). The declaration is KEY-ONLY on
// purpose: prod callback_url/callback_secret are admin-configured infra-side and
// the ensure face is sparse, so a {key}-only item can never clobber them. The
// call is declarative and idempotent — the same slice twice converges to
// all-`unchanged` — so it is safe to re-run on every boot.
//
// ADDING A NEW CONTENT TYPE = append its kind here (and wire its gate scan/
// check calls, report button, and/or enforcer). This slice is the one place
// infra must learn a new forum kind from.
var CanonicalSubjectKinds = []string{
	// Scanned + reported + enforced (the same kind across all three paths).
	SubjectKindTopic,
	SubjectKindReply,
	SubjectKindTopicComment,
	// Scan-only (async post-commit shadow scan; no report button / local enforcement).
	SubjectKindTopicPoll,
	SubjectKindGalgameRating,
	SubjectKindGalgameResource,
	SubjectKindGalgameCollection,
	SubjectKindGalgameQuiz,
	SubjectKindGalgameQuizAnswer,
	SubjectKindToolset,
	SubjectKindToolsetResource,
	// Report / enforcement-only (never shadow-scanned; see the const block above).
	SubjectKindGalgameComment,
	SubjectKindGalgame,
	SubjectKindUser,
}

// CanonicalSubjectKindItems adapts CanonicalSubjectKinds to the ensure payload
// as key-only items (the sparse declaration; see CanonicalSubjectKinds).
func CanonicalSubjectKindItems() []trustclient.EnsureSubjectKindItem {
	items := make([]trustclient.EnsureSubjectKindItem, len(CanonicalSubjectKinds))
	for i, k := range CanonicalSubjectKinds {
		items[i] = trustclient.EnsureSubjectKindItem{Key: k}
	}
	return items
}
