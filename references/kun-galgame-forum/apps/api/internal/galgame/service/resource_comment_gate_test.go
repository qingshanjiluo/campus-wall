package service

import "testing"

// TestQuizGateLocked pins the spoiler gate's truth table. The rule protects quiz
// answers from being leaked by the comment area, so every row here is a content
// guarantee, not a preference.
func TestQuizGateLocked(t *testing.T) {
	cases := []struct {
		name         string
		hideGalgame  bool
		spoilerLevel string
		isAuthor     bool
		hasAnswered  bool
		want         bool
	}{
		// Nothing to conceal → never gated, whoever is looking.
		{"open quiz, stranger", false, "none", false, false, false},
		{"open quiz, empty spoiler level (pre-046 row)", false, "", false, false, false},
		{"open quiz, anonymous", false, "none", false, false, false},

		// Hidden galgames → gated until answered.
		{"hidden galgame, stranger", true, "none", false, false, true},
		{"hidden galgame, answered", true, "none", false, true, false},
		{"hidden galgame, author", true, "none", true, false, false},

		// A spoiler level alone is enough to conceal.
		{"portion spoiler, stranger", false, "portion", false, false, true},
		{"serious spoiler, stranger", false, "serious", false, false, true},
		{"portion spoiler, answered", false, "portion", false, true, false},
		{"serious spoiler, author", false, "serious", true, false, false},

		// Both signals set behave like either one.
		{"hidden + serious, stranger", true, "serious", false, false, true},
		{"hidden + serious, answered", true, "serious", false, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quizGateLocked(tc.hideGalgame, tc.spoilerLevel, tc.isAuthor, tc.hasAnswered)
			if got != tc.want {
				t.Errorf("quizGateLocked(%v, %q, author=%v, answered=%v) = %v, want %v",
					tc.hideGalgame, tc.spoilerLevel, tc.isAuthor, tc.hasAnswered, got, tc.want)
			}
		})
	}
}
