// Package dlsite builds DLsite affiliate purchase links for the 补票 (buy-legit)
// prompt.
//
// kungal has an affiliate partnership with DLsite. A galgame that resolves to a
// DLsite work number gets a direct purchase link in the 补票 prompt, instead of
// only being pointed at the 制作商 section.
//
// The work number arrives from the catalog galgame read face as `refs.dlsite`
// (the value is the workno verbatim). Catalog sources it by direct galgame_id
// lookup, so it is present for r18 and claimed works alike — which matters,
// because 98.3% of kungal's DLsite-mapped titles are r18.
package dlsite

import "strings"

// worknoPlaceholder is what LinkTemplate carries where the id goes.
const worknoPlaceholder = "{workno}"

// worknoPrefixes are the DLsite id families kungal can link. Deliberately a
// whitelist rather than "anything non-empty": the workno is interpolated into an
// outbound partner URL, so an unexpected value must produce NO link rather than a
// malformed one that burns an affiliate click.
//
//   - RJ — doujin (maniax)
//   - VJ — commercial games (pro)
//
// AJ is a real DLsite family and is provisioned in the upstream crawler, but that
// sweep has never run, so no AJ id can reach us today. It is left out until one
// actually does; add it here (no other change) when that happens.
var worknoPrefixes = []string{"RJ", "VJ"}

// Link renders the purchase URL for a workno, or "" when no link should be shown
// (feature unconfigured, missing workno, or a shape we do not recognise).
//
// The digit width is NOT normalised. DLsite ids are natively 6- or 8-digit
// depending on the era of the work (each work has exactly one canonical form),
// and kungal's own data is 97% 6-digit — zero-padding them would invent ids that
// do not exist.
func Link(template, workno string) string {
	if template == "" || !ValidWorkno(workno) {
		return ""
	}
	return strings.ReplaceAll(template, worknoPlaceholder, workno)
}

// LinkFor renders the purchase URL for a galgame, resolving its workno in
// precedence order:
//
//  1. refsWorkno — the catalog's refs.dlsite, an exact verified identity anchor
//     that updates itself. Always wins where present.
//  2. the vendored whitelist (verified.go) — infra-audited pairs recovered from
//     user-contributed galgame_link URLs, covering ~1,400 games refs does not.
//
// "" when neither has one, or the affiliate template is unconfigured.
func LinkFor(template string, galgameID int, refsWorkno string) string {
	if url := Link(template, refsWorkno); url != "" {
		return url
	}
	return Link(template, VerifiedWorkno(galgameID))
}

// ValidWorkno reports whether s looks like a DLsite work number kungal links:
// a known prefix followed by at least one digit, and digits only after it.
func ValidWorkno(s string) bool {
	digits, ok := trimKnownPrefix(s)
	if !ok || digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// trimKnownPrefix strips a recognised family prefix, reporting whether one
// matched. The comparison is case-sensitive: DLsite ids are upper-case, and
// accepting other casings would let a mis-cased value through into a partner URL.
func trimKnownPrefix(s string) (string, bool) {
	for _, p := range worknoPrefixes {
		if rest, found := strings.CutPrefix(s, p); found {
			return rest, true
		}
	}
	return "", false
}
