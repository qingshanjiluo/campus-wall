package dlsite

import (
	"strings"
	"testing"
)

// tmpl mirrors the partner-supplied template shape.
const tmpl = "https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/locale/zh_CN/id/{workno}.html/?locale=zh_CN"

func TestValidWorkno(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		// The two families kungal links, in both native digit widths.
		{"RJ297925", true},   // doujin, 6-digit
		{"VJ013975", true},   // commercial, 6-digit
		{"RJ01005286", true}, // 8-digit era
		{"VJ01005286", true},

		// AJ is a real DLsite family but never reaches us (crawler sweep never
		// ran); until one does, it must not produce a link.
		{"AJ001234", false},

		// Not DLsite ids at all.
		{"", false},
		{"RJ", false},        // prefix with no number
		{"297925", false},    // bare number, no family
		{"XX123456", false},  // unknown family
		{"rj297925", false},  // lower case — DLsite ids are upper case
		{"RJ12 34", false},   // embedded space
		{"RJ123456/", false}, // trailing punctuation (path-injection shaped)
		{"RJ12a456", false},  // non-digit in the number
	}
	for _, tc := range cases {
		if got := ValidWorkno(tc.in); got != tc.want {
			t.Errorf("ValidWorkno(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestLink(t *testing.T) {
	t.Run("interpolates the workno verbatim", func(t *testing.T) {
		got := Link(tmpl, "VJ013975")
		want := "https://dlaf.jp/soft/dlaf/=/t/s/link/work/aid/kungal/locale/zh_CN/id/VJ013975.html/?locale=zh_CN"
		if got != want {
			t.Errorf("Link() = %q, want %q", got, want)
		}
	})

	t.Run("does not pad the digit width", func(t *testing.T) {
		// 6-digit ids are canonical for older works; padding would invent an id
		// that does not exist on DLsite.
		if got := Link(tmpl, "VJ013975"); !strings.Contains(got, "id/VJ013975.html") {
			t.Errorf("Link() rewrote the workno: %q", got)
		}
	})

	t.Run("empty template disables the feature", func(t *testing.T) {
		if got := Link("", "RJ297925"); got != "" {
			t.Errorf("Link() with no template = %q, want \"\"", got)
		}
	})

	t.Run("an unrecognised workno yields no link", func(t *testing.T) {
		// A malformed id must produce NO link rather than a broken partner URL.
		for _, bad := range []string{"", "AJ001234", "rj297925", "RJ123456/"} {
			if got := Link(tmpl, bad); got != "" {
				t.Errorf("Link(%q) = %q, want \"\"", bad, got)
			}
		}
	})
}

// TestVerifiedWhitelist checks the vendored snapshot actually loaded — a
// mis-vendored (empty / truncated) file would silently disable the fallback for
// every game rather than fail loudly.
func TestVerifiedWhitelist(t *testing.T) {
	// infra's audit delivered 4,320 verified pairs out of 5,077 parsed; they
	// collapse to 4,319 ids because galgame 4156 verifies against two releases.
	const want = 4319
	if got := VerifiedCount(); got != want {
		t.Errorf("VerifiedCount() = %d, want %d (verified.tsv mis-vendored?)", got, want)
	}

	t.Run("a conflicted game uses infra's pinned ruling", func(t *testing.T) {
		// 4156 verifies against RJ088116 and RJ090411. RJ088116 is DELISTED in the
		// DLsite mirror (empty work_name); RJ090411 is the live product. Title
		// similarity — all the audit checked — cannot tell them apart, so infra's
		// mirror ruling is pinned. A "lower workno wins" tie-break picks the dead
		// one, which is exactly the bug this guards.
		if got := VerifiedWorkno(4156); got != "RJ090411" {
			t.Errorf("VerifiedWorkno(4156) = %q, want RJ090411 (infra pinned)", got)
		}
	})

	t.Run("header row is not parsed as a pair", func(t *testing.T) {
		if got := VerifiedWorkno(0); got != "" {
			t.Errorf("VerifiedWorkno(0) = %q, want \"\"", got)
		}
	})

	t.Run("an unknown galgame has no entry", func(t *testing.T) {
		if got := VerifiedWorkno(999999999); got != "" {
			t.Errorf("VerifiedWorkno(999999999) = %q, want \"\"", got)
		}
	})

	t.Run("every entry is a well-formed workno", func(t *testing.T) {
		// The whitelist is trusted for the PAIRING, not the id's shape.
		for id := 1; id <= 70000; id++ {
			if wn := VerifiedWorkno(id); wn != "" && !ValidWorkno(wn) {
				t.Fatalf("galgame %d maps to malformed workno %q", id, wn)
			}
		}
	})
}

// TestLinkForPrecedence pins that the canonical catalog anchor always beats the
// vendored snapshot, and that the snapshot only fills gaps.
func TestLinkForPrecedence(t *testing.T) {
	// galgame 4 is on the whitelist as VJ013550.
	const whitelisted = 4

	t.Run("refs wins when present", func(t *testing.T) {
		got := LinkFor(tmpl, whitelisted, "RJ297925")
		if !strings.Contains(got, "RJ297925") {
			t.Errorf("LinkFor() = %q, want the refs workno to win", got)
		}
	})

	t.Run("whitelist fills the gap when refs is absent", func(t *testing.T) {
		got := LinkFor(tmpl, whitelisted, "")
		if !strings.Contains(got, "VJ013550") {
			t.Errorf("LinkFor() = %q, want the whitelisted workno", got)
		}
	})

	t.Run("neither source yields no link", func(t *testing.T) {
		if got := LinkFor(tmpl, 999999999, ""); got != "" {
			t.Errorf("LinkFor() = %q, want \"\"", got)
		}
	})

	t.Run("unconfigured template disables both paths", func(t *testing.T) {
		if got := LinkFor("", whitelisted, "RJ297925"); got != "" {
			t.Errorf("LinkFor() with no template = %q, want \"\"", got)
		}
	})
}

// TestResolveVerifiedConflicts pins the conflict policy itself: one workno is
// taken as-is, several are resolved ONLY by a pinned ruling, and an unruled
// conflict yields no link rather than a guess.
func TestResolveVerifiedConflicts(t *testing.T) {
	got := resolveVerified(map[int][]string{
		1:    {"VJ000001"},                         // unambiguous
		2:    {"RJ000002", "RJ000003"},             // conflict, not pinned
		3:    {"RJ000004", "RJ000005", "RJ000006"}, // 3-way conflict, not pinned
		4156: {"RJ088116", "RJ090411"},             // conflict WITH a pinned ruling
	})

	if got[1] != "VJ000001" {
		t.Errorf("single workno = %q, want VJ000001", got[1])
	}
	if _, ok := got[2]; ok {
		t.Errorf("unpinned conflict resolved to %q, want no entry (never guess)", got[2])
	}
	if _, ok := got[3]; ok {
		t.Errorf("unpinned 3-way conflict resolved to %q, want no entry", got[3])
	}
	if got[4156] != "RJ090411" {
		t.Errorf("pinned conflict = %q, want RJ090411", got[4156])
	}
}
