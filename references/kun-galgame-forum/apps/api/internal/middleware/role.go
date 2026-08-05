package middleware

import (
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/role"

	"github.com/gofiber/fiber/v3"
)

// RequireModerator gates a route to holders of the management capability
// (moderator ⊂ admin ⊂ ren per docs/oauth/11-roles.md). 403 otherwise.
func RequireModerator() fiber.Handler {
	return func(c fiber.Ctx) error {
		user := GetUser(c)
		if user == nil {
			return response.Error(c, errors.ErrAuthExpired())
		}
		if !role.CanModerate(user.Roles) {
			return response.Error(c, errors.ErrForbidden("您没有权限进行此操作"))
		}
		return c.Next()
	}
}

// RequireAdmin gates a route to holders of the site-administration capability
// (admin ⊂ ren). 403 otherwise.
func RequireAdmin() fiber.Handler {
	return func(c fiber.Ctx) error {
		user := GetUser(c)
		if user == nil {
			return response.Error(c, errors.ErrAuthExpired())
		}
		if !role.CanAdminister(user.Roles) {
			return response.Error(c, errors.ErrForbidden("您没有权限进行此操作"))
		}
		return c.Next()
	}
}

// RequirePermission gates a route to holders of a named PURE-FORUM permission
// (pkg/perm — the forum's own resolver is the authoritative boundary for these
// operations). Same shape as RequireModerator: nil user → auth-expired, missing
// permission → 403. INFRA-PROXY operations do NOT use this — they stay on
// RequireModerator/RequireAdmin with a mirror comment.
func RequirePermission(p perm.Permission) fiber.Handler {
	return func(c fiber.Ctx) error {
		user := GetUser(c)
		if user == nil {
			return response.Error(c, errors.ErrAuthExpired())
		}
		if !perm.CanUser(user.ID, user.Roles, p) {
			return response.Error(c, errors.ErrForbidden("您没有权限进行此操作"))
		}
		return c.Next()
	}
}
