package service

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"kun-galgame-api/internal/admin/dto"
	"kun-galgame-api/internal/admin/model"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
	"kun-galgame-api/pkg/userclient"
)

// userOverrideStore is the persistence seam for per-user overrides — a small
// interface so tests can supply an in-memory fake without a database.
type userOverrideStore interface {
	ListForUser(ctx context.Context, userID int) ([]model.UserPermissionOverride, error)
	ReplaceForUser(ctx context.Context, userID int, rows []model.UserPermissionOverride, operatorUID int) error
}

// userLookup resolves a target user's roles from OAuth (the role source of
// truth). Only User is needed; the concrete userclient.Client satisfies it.
type userLookup interface {
	User(ctx context.Context, id int) (userclient.User, bool, error)
}

// UserPermissionService serves the per-user permission editor (view + replace)
// and the current-user /perm/mine read. Writes go through the shared reloader so
// both pkg/perm layers converge immediately (write-through).
type UserPermissionService struct {
	repo       userOverrideStore
	userClient userLookup
	reload     reloader
}

func NewUserPermissionService(repo userOverrideStore, userClient userLookup, reload reloader) *UserPermissionService {
	return &UserPermissionService{repo: repo, userClient: userClient, reload: reload}
}

// View returns the per-user permission read model: the target's roles, their
// role-derived effective set (the reference the panel diffs against), their
// stored personal overrides, and the effective set with those deltas applied.
func (s *UserPermissionService) View(ctx context.Context, uid int) (dto.UserPermissionView, *errors.AppError) {
	if uid <= 0 {
		return dto.UserPermissionView{}, errors.ErrBadRequest("非法的用户 ID")
	}
	roles, appErr := s.targetRoles(ctx, uid)
	if appErr != nil {
		return dto.UserPermissionView{}, appErr
	}
	rows, err := s.repo.ListForUser(ctx, uid)
	if err != nil {
		return dto.UserPermissionView{}, errors.ErrInternal("读取用户权限失败")
	}
	return buildUserView(uid, roles, rows), nil
}

// ReplaceOverrides validates and atomically replaces one user's personal override
// set, re-Loads so the change is live at once (write-through), and returns the
// fresh view. Validation failures return a user-facing Chinese 400. operatorRoles
// are the editing admin's role claims — the delegation guard (rank + possession)
// is judged against them.
func (s *UserPermissionService) ReplaceOverrides(ctx context.Context, operatorUID int, operatorRoles []string, uid int, items []dto.ReplaceOverrideItem) (dto.UserPermissionView, *errors.AppError) {
	if uid <= 0 {
		return dto.UserPermissionView{}, errors.ErrBadRequest("非法的用户 ID")
	}
	roles, appErr := s.targetRoles(ctx, uid)
	if appErr != nil {
		return dto.UserPermissionView{}, appErr
	}
	currentRows, err := s.repo.ListForUser(ctx, uid)
	if err != nil {
		return dto.UserPermissionView{}, errors.ErrInternal("读取用户权限失败")
	}
	if appErr := validateUserReplace(operatorUID, operatorRoles, roles, items, currentRows); appErr != nil {
		return dto.UserPermissionView{}, appErr
	}

	if err := s.repo.ReplaceForUser(ctx, uid, userItemsToRows(items), operatorUID); err != nil {
		return dto.UserPermissionView{}, errors.ErrInternal("保存用户权限失败")
	}
	// Write-through: a Load failure only means the in-memory table is briefly stale
	// (the refresher converges), so we warn and still return the fresh view.
	if err := s.reload.Load(ctx); err != nil {
		slog.Warn("写入后刷新用户权限覆盖失败, 稍后自动收敛", "error", err)
	}

	rows, err := s.repo.ListForUser(ctx, uid)
	if err != nil {
		return dto.UserPermissionView{}, errors.ErrInternal("读取用户权限失败")
	}
	return buildUserView(uid, roles, rows), nil
}

// MyPermissions returns the caller's own effective permission list (role-derived
// set with personal deltas applied), in catalog order. Feeds FE visibility only.
func (s *UserPermissionService) MyPermissions(uid int, roles []string) dto.MyPermissionsResponse {
	return dto.MyPermissionsResponse{Permissions: permsToStrings(perm.EffectiveForUser(uid, roles))}
}

// targetRoles resolves the target user's roles from OAuth, reusing the purge
// guard's pattern. It fails CLOSED: an OAuth lookup error OR a nonexistent user
// both abort with the same guard message — we never adjust permissions for an
// identity we cannot verify.
func (s *UserPermissionService) targetRoles(ctx context.Context, uid int) ([]string, *errors.AppError) {
	u, found, err := s.userClient.User(ctx, uid)
	if err != nil || !found {
		return nil, errors.ErrBadRequest("无法核验目标用户身份, 已中止")
	}
	return u.Roles, nil
}

// validateUserReplace enforces every guard rail on a proposed per-user replace:
//   - the target must not hold `ren` (ren holders are pinned to the full catalog)
//   - RANK: the operator's rank must strictly exceed the target user's rank
//   - each permission must be a known catalog key
//   - each effect must be grant or revoke
//   - no duplicate permission in the request
//   - no NO-OP judged against the user's ROLE-DERIVED effective set: granting a
//     key their roles already effectively hold, or revoking a key their roles do
//     not hold, is a 400 (the FE computes true personal deltas).
//   - POSSESSION: every row the request adds/removes/changes vs the user's stored
//     personal rows must be a key the operator's own effective set holds.
//
// `roles` are the TARGET user's roles; `current` their stored personal rows.
func validateUserReplace(operatorUID int, operatorRoles, roles []string, items []dto.ReplaceOverrideItem, current []model.UserPermissionOverride) *errors.AppError {
	if hasRen(roles) {
		return errors.ErrBadRequest("ren 持有者恒持全部权限, 不可调整")
	}
	// Rank: an operator may only edit a user whose rank is strictly below theirs.
	if perm.Rank(operatorRoles) <= perm.Rank(roles) {
		return errors.ErrBadRequest("不可编辑与自身同级或更高的用户")
	}
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		p := perm.Permission(it.Permission)
		if !perm.IsKnownPermission(p) {
			return errors.ErrBadRequest("未知的权限: " + it.Permission)
		}
		if it.Effect != perm.EffectGrant && it.Effect != perm.EffectRevoke {
			return errors.ErrBadRequest("非法的调整类型: " + it.Effect + " (仅支持 grant / revoke)")
		}
		if seen[it.Permission] {
			return errors.ErrBadRequest("权限 " + it.Permission + " 重复出现")
		}
		seen[it.Permission] = true

		// The reference is the target's role-derived set: perm.Can consults the
		// installed role baseline ± role overrides (personal deltas are NOT part of
		// the reference — the point is to compute a delta against the role set).
		roleHolds := perm.Can(roles, p)
		if it.Effect == perm.EffectGrant && roleHolds {
			return errors.ErrBadRequest("权限 " + it.Permission + " 已由该用户的角色授予, 无需重复授予")
		}
		if it.Effect == perm.EffectRevoke && !roleHolds {
			return errors.ErrBadRequest("权限 " + it.Permission + " 不在该用户的角色权限内, 无需撤销")
		}
	}

	// Possession: only rows this request actually changes vs the user's stored
	// personal rows are judged, and each changed key must be one the operator
	// themselves hold. Carried-over rows pass untouched.
	if key, ok := possessionOffender(
		effectMapFromUserRows(current),
		effectMapFromItems(items),
		operatorEffectiveSet(operatorUID, operatorRoles),
	); !ok {
		return errors.ErrBadRequest("不可增删自己未持有的权限: " + key)
	}
	return nil
}

// hasRen reports whether a role set includes ren (the pinned-full-catalog role).
func hasRen(roles []string) bool {
	return slices.Contains(roles, "ren")
}

// ──────────────────────────────────────────
// Read-model construction
// ──────────────────────────────────────────

// buildUserView assembles the per-user read model. role_effective is the pure
// role-derived set (perm.Can over the catalog); effective applies the stored
// personal deltas on top (perm.EffectiveForUser). Both are catalog-ordered and
// non-null; the ren pin is honored by EffectiveForUser.
func buildUserView(uid int, roles []string, rows []model.UserPermissionOverride) dto.UserPermissionView {
	roleEffective := make([]string, 0, len(perm.Catalog()))
	for _, p := range perm.Catalog() {
		if perm.Can(roles, p) {
			roleEffective = append(roleEffective, string(p))
		}
	}
	effective := permsToStrings(perm.EffectiveForUser(uid, roles))
	return dto.UserPermissionView{
		UserID:        uid,
		Roles:         nonNilStrings(roles),
		RoleEffective: roleEffective,
		Overrides:     userOverridesToDTO(rows),
		Effective:     effective,
	}
}

// userOverridesToDTO renders a user's rows in catalog order (a stray duplicate
// keeps the last row), always as a non-nil slice.
func userOverridesToDTO(rows []model.UserPermissionOverride) []dto.UserPermissionOverride {
	byPerm := make(map[perm.Permission]model.UserPermissionOverride, len(rows))
	for _, r := range rows {
		byPerm[perm.Permission(r.Permission)] = r
	}
	out := make([]dto.UserPermissionOverride, 0, len(rows))
	for _, p := range perm.Catalog() {
		if r, ok := byPerm[p]; ok {
			out = append(out, dto.UserPermissionOverride{
				Permission: r.Permission,
				Effect:     r.Effect,
				UpdatedBy:  r.UpdatedBy,
				UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
			})
		}
	}
	return out
}

// userItemsToRows builds the model rows to persist (user_id/updated_by/at are
// stamped by the repository inside the transaction).
func userItemsToRows(items []dto.ReplaceOverrideItem) []model.UserPermissionOverride {
	out := make([]model.UserPermissionOverride, 0, len(items))
	for _, it := range items {
		out = append(out, model.UserPermissionOverride{
			Permission: it.Permission,
			Effect:     it.Effect,
		})
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
