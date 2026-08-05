package gate

import "testing"

// TestCanonicalSubjectKindsCoversGateConstants pins the invariant that the
// declarative ensure slice contains every subject-kind constant the gate uses.
// A gate scan/report/enforce path against a kind absent from CanonicalSubjectKinds
// would 422 at runtime (fail-loud registry), so this catches the "added a kind
// but forgot to declare it" mistake at compile+test time.
func TestCanonicalSubjectKindsCoversGateConstants(t *testing.T) {
	set := make(map[string]bool, len(CanonicalSubjectKinds))
	for _, k := range CanonicalSubjectKinds {
		if set[k] {
			t.Fatalf("duplicate kind in CanonicalSubjectKinds: %q", k)
		}
		set[k] = true
	}

	// Every SubjectKind* constant defined for the gate (scan.go + kinds.go).
	required := []string{
		SubjectKindTopic,
		SubjectKindReply,
		SubjectKindTopicComment,
		SubjectKindTopicPoll,
		SubjectKindGalgameRating,
		SubjectKindGalgameResource,
		SubjectKindGalgameCollection,
		SubjectKindGalgameQuiz,
		SubjectKindGalgameQuizAnswer,
		SubjectKindToolset,
		SubjectKindToolsetResource,
		SubjectKindGalgameComment,
		SubjectKindGalgame,
		SubjectKindUser,
	}
	for _, k := range required {
		if !set[k] {
			t.Errorf("CanonicalSubjectKinds is missing gate constant %q", k)
		}
	}

	// The slice must be exactly the constant set — nothing undeclared, nothing
	// stray. If a new kind is added, add it to both places (this test is the pin).
	if len(CanonicalSubjectKinds) != len(required) {
		t.Errorf("CanonicalSubjectKinds has %d kinds, required set has %d — keep them in lockstep",
			len(CanonicalSubjectKinds), len(required))
	}

	// The ensure payload adapter preserves order and key-only shape.
	items := CanonicalSubjectKindItems()
	if len(items) != len(CanonicalSubjectKinds) {
		t.Fatalf("adapter length %d != slice length %d", len(items), len(CanonicalSubjectKinds))
	}
	for i, it := range items {
		if it.Key != CanonicalSubjectKinds[i] {
			t.Errorf("adapter item %d = %q, want %q", i, it.Key, CanonicalSubjectKinds[i])
		}
	}
}
