package perm

// This file adds the DELEGATION rank ladder used only by the admin permission
// editors' delegation guard (see internal/admin/service): an operator may edit a
// subject strictly BELOW their own rank. It is a LOCAL ordering for delegation
// authority, NOT a general role tier — enforcement everywhere else is a capability
// over the claim set (Can / CanUser), never a number. It lives here in pkg/perm
// because it is pure rank algebra with no storage or HTTP dependency.

// roleRank is the delegation ladder, high → low: ren=4, admin=3, moderator=2,
// creator=1. Any other/absent role (including the implicit `user`) ranks 0.
var roleRank = map[string]int{
	roleRen:     4,
	"admin":     3,
	"moderator": 2,
	"creator":   1,
}

// RoleRank returns the delegation rank of a SINGLE role (0 if unknown). See the
// order in roleRank.
func RoleRank(role string) int { return roleRank[role] }

// Rank returns the delegation rank of a role SET: the max rank over its claims,
// or 0 for an empty/rankless set. An operator's rank is Rank(operatorRoles); a
// target user's rank is Rank(targetRoles).
func Rank(roles []string) int {
	best := 0
	for _, r := range roles {
		if v := roleRank[r]; v > best {
			best = v
		}
	}
	return best
}
