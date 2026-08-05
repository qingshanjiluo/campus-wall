// Package role is kungal's binding to the OAuth-owned role contract
// (docs/oauth/11-roles.md — Tier A, the single authoritative source).
//
// Roles arrive as an UNORDERED SET of name strings in the access-token `roles`
// claim. There is no numeric tier and no ordering in the wire format. Two facts
// from the contract shape everything here:
//
//   - `user` is the IMPLICIT default identity and NEVER appears in the claim —
//     a plain logged-in user has an empty set. Detect login by holding a valid
//     session, NEVER by membership of "user" (there is deliberately no User
//     constant below).
//   - Authority splits into two ORTHOGONAL axes:
//     management (inclusive):  moderator ⊂ admin ⊂ ren
//     publishing (orthogonal): creator  (no moderation/admin power)
//
// All authorization decisions express a CAPABILITY (CanModerate / CanAdminister
// / IsCreator) over the claim set — never a role number, never an ordered rank.
package role

import "slices"

// Role-name constants — the exact claim strings (contract §1). The retired
// site-wide alias the IdP never signs is intentionally absent.
const (
	Creator   = "creator"
	Moderator = "moderator"
	Admin     = "admin"
	Ren       = "ren"
)

// Has reports whether the claim set contains an exact role name.
func Has(roles []string, name string) bool {
	return slices.Contains(roles, name)
}

// CanModerate reports the management capability "act on OTHER users' content"
// (edit/delete/pin/hide/review). Inclusive per contract §3.1/§4.2: admin and
// ren both subsume moderator, so a holder of any of the three qualifies.
func CanModerate(roles []string) bool {
	return Has(roles, Moderator) || Has(roles, Admin) || Has(roles, Ren)
}

// CanAdminister reports the management capability "site / user administration"
// (bans, site & client config, role grants, taxonomy edits). ren subsumes
// admin, so either qualifies.
func CanAdminister(roles []string) bool {
	return Has(roles, Admin) || Has(roles, Ren)
}

// IsCreator reports the orthogonal "trusted publisher" capability (direct
// galgame publish). It grants NO moderation or admin power (contract §3.1).
func IsCreator(roles []string) bool {
	return Has(roles, Creator)
}

// Union returns roles ∪ siteRoles as the deduplicated EFFECTIVE role set the
// capability functions operate on (contract docs/oauth/12-site-roles.md §5.1:
// "effective = roles ∪ site_roles, feed your EXISTING capability functions —
// no new decision path"). site_roles is an additive, same-site extension;
// because the IdP forbids admin/ren in it (§3/§5.3), unioning can only ADD
// moderator/creator/custom names — it can never reach an admin-only capability.
// Inputs are left untouched (roles is cloned before extending).
func Union(roles, siteRoles []string) []string {
	if len(siteRoles) == 0 {
		return roles
	}
	out := slices.Clone(roles)
	for _, r := range siteRoles {
		if !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}
