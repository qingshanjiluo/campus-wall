package dlsite

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
)

// verified.tsv is the infra-audited galgame_id → DLsite workno whitelist
// (`galgame_id \t workno \t score \t matched_field`, header row first).
//
// WHY A VENDORED SNAPSHOT. The canonical source is the catalog's refs.dlsite,
// which arrives on the galgame read faces and updates itself. But refs covers
// 3,701 of kungal's games while the DLsite URLs users already contributed to
// galgame_link cover 5,077 — so ~1,400 games would show no purchase link despite
// having a perfectly good DLsite id sitting in their links.
//
// Those raw links cannot be trusted blindly: they are user/VNDB-contributed, and
// infra's audit found real mismatches — links pointing at a budget re-release
// (まいてつ -pure station- → 普及版), another platform (Kanon → 【Android版】), or a
// multi-title compilation (沙耶の唄 → Nitro The Best! Vol.2). Sending a buyer to
// the wrong product is worse than showing no button, so kungal does NOT parse
// galgame_link itself. It ships only the pairs infra verified by title match
// (4,320 of 5,077); the 757 suspicious ones (429 mirror-coverage gaps, 328 title
// mismatches) are deliberately absent.
//
// This is a BRIDGE, not a permanent source. refs.dlsite always wins where it
// exists; the whitelist only fills gaps, and it does not grow — a game that gets
// a DLsite link after this snapshot is picked up when refs covers it. Once refs
// covers the whitelist, this file and its lookup can be deleted outright.
//
//go:embed verified.tsv
var verifiedTSV string

var (
	verifiedOnce sync.Once
	verifiedMap  map[int]string
)

// VerifiedWorkno returns the audited workno for a galgame id, or "" when the id
// is not on the whitelist (never verified, or verified as suspicious).
func VerifiedWorkno(galgameID int) string {
	verifiedOnce.Do(loadVerified)
	return verifiedMap[galgameID]
}

// VerifiedCount reports how many pairs the whitelist holds — a startup log line
// makes a mis-vendored (empty / truncated) file obvious instead of silently
// turning the fallback off.
func VerifiedCount() int {
	verifiedOnce.Do(loadVerified)
	return len(verifiedMap)
}

// pinnedWorkno records infra's mirror-checked rulings for galgame ids the
// whitelist verifies against MORE THAN ONE workno.
//
// Such a conflict cannot be resolved locally. The audit that produced the
// whitelist compared TITLE SIMILARITY only — it never checked whether the product
// page still exists, so "both verified" does not mean "both linkable". galgame
// 4156 is the proof: its two entries are RJ088116 and RJ090411, both scored 1.00
// on the Japanese title, but RJ088116 has an empty work_name in the DLsite mirror
// (delisted / placeholder) while RJ090411 is the live 《喪失のMarried Life》. A
// "prefer the lower workno" rule — the obvious tie-break, and what this code did
// first — picks the dead one.
//
// So conflicts are DATA, not a heuristic: resolved ids are pinned here with their
// provenance, and an unpinned conflict yields NO link at all. Ask infra to check
// the mirror before adding a row.
var pinnedWorkno = map[int]string{
	// infra mirror check 2026-07-26: RJ088116 is delisted, RJ090411 is live.
	4156: "RJ090411",
}

// loadVerified parses the embedded TSV once. Malformed rows are skipped rather
// than fatal: a bad line must not take the process down, and the worst case is
// one game losing its purchase link. The workno still goes through ValidWorkno —
// the whitelist is trusted for the PAIRING, not for the id's shape.
func loadVerified() {
	pairs := make(map[int][]string, 4500)
	for line := range strings.SplitSeq(strings.TrimSpace(verifiedTSV), "\n") {
		cols := strings.Split(strings.TrimSpace(line), "\t")
		if len(cols) < 2 {
			continue
		}
		id, err := strconv.Atoi(cols[0]) // skips the header row too
		if err != nil || id <= 0 || !ValidWorkno(cols[1]) {
			continue
		}
		pairs[id] = append(pairs[id], cols[1])
	}
	verifiedMap = resolveVerified(pairs)
}

// resolveVerified collapses each galgame's verified worknos to the one kungal
// links. Exactly one → use it. More than one → only a pinned ruling counts;
// otherwise the game gets no link, because guessing risks sending a buyer to a
// delisted product. Pure, so the conflict rules are unit-testable without the
// embedded file.
func resolveVerified(pairs map[int][]string) map[int]string {
	out := make(map[int]string, len(pairs))
	for id, worknos := range pairs {
		switch {
		case len(worknos) == 1:
			out[id] = worknos[0]
		default:
			if pinned, ok := pinnedWorkno[id]; ok {
				out[id] = pinned
			}
		}
	}
	return out
}
