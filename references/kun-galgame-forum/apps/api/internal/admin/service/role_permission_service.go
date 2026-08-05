package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"kun-galgame-api/internal/admin/dto"
	"kun-galgame-api/internal/admin/model"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/perm"
)

// overrideStore is the persistence seam the service needs — a small interface so
// tests can supply an in-memory fake without a database.
type overrideStore interface {
	ListAll(ctx context.Context) ([]model.RolePermissionOverride, error)
	ReplaceForRole(ctx context.Context, role string, rows []model.RolePermissionOverride, operatorUID int) error
}

// reloader refreshes the live pkg/perm override tables after a write. The
// PermissionOverrideSync satisfies it — routing every write-through (role AND
// user) through the ONE Load path keeps both layers converging uniformly.
type reloader interface {
	Load(ctx context.Context) error
}

// RolePermissionService serves the admin role→permission matrix and validates +
// applies edits (write-through via the shared reloader). Boot/refresh loading of
// the effective table lives in PermissionOverrideSync (the single Load path).
type RolePermissionService struct {
	repo   overrideStore
	reload reloader
}

func NewRolePermissionService(repo overrideStore, reload reloader) *RolePermissionService {
	return &RolePermissionService{repo: repo, reload: reload}
}

// editableRoles are the only roles an admin may override. ren is pinned to the
// full catalog (never stored); user is implicit in the OAuth contract (never a
// claim), so a 'user' override would be dead configuration on both sides.
var editableRoles = map[string]bool{"creator": true, "moderator": true, "admin": true}

// matrixRoles is the display order of the matrix (ren last, pinned).
var matrixRoles = []string{"creator", "moderator", "admin", "ren"}

// Matrix returns the full read model for the admin editor.
func (s *RolePermissionService) Matrix(ctx context.Context) (dto.RolePermissionMatrix, error) {
	rows, err := s.repo.ListAll(ctx)
	if err != nil {
		return dto.RolePermissionMatrix{}, err
	}
	return buildMatrix(rows), nil
}

// ReplaceOverrides validates and atomically replaces one role's override set,
// then re-Loads so the change takes effect immediately (write-through). It
// returns the fresh matrix. Validation failures return a user-facing Chinese
// 400. operatorRoles are the editing admin's role claims — the delegation guard
// (rank + possession) is judged against them.
func (s *RolePermissionService) ReplaceOverrides(ctx context.Context, operatorUID int, operatorRoles []string, role string, items []dto.ReplaceOverrideItem) (dto.RolePermissionMatrix, *errors.AppError) {
	current, err := s.repo.ListAll(ctx)
	if err != nil {
		return dto.RolePermissionMatrix{}, errors.ErrInternal("读取角色权限失败")
	}
	if appErr := validateReplace(operatorUID, operatorRoles, role, items, current); appErr != nil {
		return dto.RolePermissionMatrix{}, appErr
	}

	if err := s.repo.ReplaceForRole(ctx, role, itemsToRows(role, items), operatorUID); err != nil {
		return dto.RolePermissionMatrix{}, errors.ErrInternal("保存角色权限失败")
	}
	// Write-through: re-Load (both layers, via the shared sync) so the edit is live
	// at once. A Load failure here only means the in-memory table is briefly stale —
	// the refresher converges — so we warn and still return a matrix built from the
	// freshly written rows.
	if err := s.reload.Load(ctx); err != nil {
		slog.Warn("写入后刷新角色权限覆盖失败, 稍后自动收敛", "error", err)
	}
	matrix, err := s.Matrix(ctx)
	if err != nil {
		return dto.RolePermissionMatrix{}, errors.ErrInternal("获取角色权限矩阵失败")
	}
	return matrix, nil
}

// validateReplace enforces every guard rail on a proposed replacement:
//   - role must be one of {creator, moderator, admin} (ren pinned, user implicit)
//   - RANK: the operator's rank must strictly exceed the target role's rank
//   - each permission must be a known catalog key
//   - each effect must be grant or revoke
//   - no duplicate permission in the request
//   - no no-op (granting a baseline key, or revoking a non-baseline key)
//   - POSSESSION: every row the request adds/removes/changes vs the stored set
//     must be a key the operator's own effective set holds
//   - containment: effective(moderator) ⊆ effective(admin) after applying
//
// `current` is every stored override row (all roles) — used to compute the
// prospective effective sets for the containment check AND the stored side of the
// possession delta.
func validateReplace(operatorUID int, operatorRoles []string, role string, items []dto.ReplaceOverrideItem, current []model.RolePermissionOverride) *errors.AppError {
	if role == "ren" {
		return errors.ErrBadRequest("ren 角色的权限被固定为全部, 不可修改")
	}
	if !editableRoles[role] {
		return errors.ErrBadRequest("不支持管理该角色, 仅可管理 creator / moderator / admin")
	}
	// Rank: an operator may only edit a role strictly below their own rank.
	if perm.Rank(operatorRoles) <= perm.RoleRank(role) {
		return errors.ErrBadRequest("不可编辑与自身同级或更高的角色 (" + role + ")")
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

		inBaseline := perm.BaselineHas(role, p)
		if it.Effect == perm.EffectGrant && inBaseline {
			return errors.ErrBadRequest("权限 " + it.Permission + " 已在 " + role + " 的默认集合中, 无需授予")
		}
		if it.Effect == perm.EffectRevoke && !inBaseline {
			return errors.ErrBadRequest("权限 " + it.Permission + " 不在 " + role + " 的默认集合中, 无需撤销")
		}
	}

	// Possession: only rows this request actually changes vs the stored set are
	// judged, and each changed key must be one the operator themselves hold. Rows
	// carried over unchanged (e.g. a grant a ren earlier set) pass untouched.
	if key, ok := possessionOffender(
		effectMapFromRoleRows(current, role),
		effectMapFromItems(items),
		operatorEffectiveSet(operatorUID, operatorRoles),
	); !ok {
		return errors.ErrBadRequest("不可增删自己未持有的权限: " + key)
	}

	// Containment: build the prospective override map (all roles' current stored
	// overrides, with the target role replaced by the request), then require every
	// effective moderator permission to also be an effective admin permission.
	prospective := rowsToOverrideMap(current)
	prospective[role] = itemsToOverrides(items)
	modEff := perm.EffectiveSet("moderator", prospective["moderator"])
	adminSet := make(map[perm.Permission]bool)
	for _, p := range perm.EffectiveSet("admin", prospective["admin"]) {
		adminSet[p] = true
	}
	var offending []string
	for _, p := range modEff {
		if !adminSet[p] {
			offending = append(offending, string(p))
		}
	}
	if len(offending) > 0 {
		sort.Strings(offending)
		return errors.ErrBadRequest("权限层级冲突: moderator 拥有但 admin 缺失的权限 — " +
			strings.Join(offending, ", ") + " (moderator 必须是 admin 的子集)")
	}
	return nil
}

// ──────────────────────────────────────────
// Read-model construction
// ──────────────────────────────────────────

func buildMatrix(rows []model.RolePermissionOverride) dto.RolePermissionMatrix {
	byRole := make(map[string][]model.RolePermissionOverride)
	for _, r := range rows {
		byRole[r.Role] = append(byRole[r.Role], r)
	}

	roles := make(map[string]dto.RolePermissionRole, len(matrixRoles))
	for _, role := range matrixRoles {
		roleRows := byRole[role]
		if role == "ren" {
			// ren is pinned and never stored; ignore any stray rows.
			roleRows = nil
		}
		roles[role] = dto.RolePermissionRole{
			Baseline:  permsToStrings(perm.Baseline(role)),
			Overrides: overridesToDTO(roleRows),
			Effective: permsToStrings(perm.EffectiveSet(role, rowsToOverrides(roleRows))),
			Locked:    role == "ren",
		}
	}
	return dto.RolePermissionMatrix{
		Catalog: permsToStrings(perm.Catalog()),
		Roles:   roles,
	}
}

// overridesToDTO renders a role's rows in catalog order (a stray duplicate keeps
// the last row), always as a non-nil slice.
func overridesToDTO(rows []model.RolePermissionOverride) []dto.RolePermissionOverride {
	byPerm := make(map[perm.Permission]model.RolePermissionOverride, len(rows))
	for _, r := range rows {
		byPerm[perm.Permission(r.Permission)] = r
	}
	out := make([]dto.RolePermissionOverride, 0, len(rows))
	for _, p := range perm.Catalog() {
		if r, ok := byPerm[p]; ok {
			out = append(out, dto.RolePermissionOverride{
				Permission: r.Permission,
				Effect:     r.Effect,
				UpdatedBy:  r.UpdatedBy,
				UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
			})
		}
	}
	return out
}

// ──────────────────────────────────────────
// Conversions
// ──────────────────────────────────────────

// rowsToOverrideMap groups all override rows into a per-role map of perm.Override
// for pkg/perm (SetOverrides filters unknown keys / ren defensively).
func rowsToOverrideMap(rows []model.RolePermissionOverride) map[string][]perm.Override {
	out := make(map[string][]perm.Override)
	for _, r := range rows {
		out[r.Role] = append(out[r.Role], perm.Override{
			Permission: perm.Permission(r.Permission),
			Effect:     r.Effect,
		})
	}
	return out
}

func rowsToOverrides(rows []model.RolePermissionOverride) []perm.Override {
	out := make([]perm.Override, 0, len(rows))
	for _, r := range rows {
		out = append(out, perm.Override{Permission: perm.Permission(r.Permission), Effect: r.Effect})
	}
	return out
}

func itemsToOverrides(items []dto.ReplaceOverrideItem) []perm.Override {
	out := make([]perm.Override, 0, len(items))
	for _, it := range items {
		out = append(out, perm.Override{Permission: perm.Permission(it.Permission), Effect: it.Effect})
	}
	return out
}

// itemsToRows builds the model rows to persist (updated_by/at are stamped by the
// repository inside the transaction).
func itemsToRows(role string, items []dto.ReplaceOverrideItem) []model.RolePermissionOverride {
	out := make([]model.RolePermissionOverride, 0, len(items))
	for _, it := range items {
		out = append(out, model.RolePermissionOverride{
			Role:       role,
			Permission: it.Permission,
			Effect:     it.Effect,
		})
	}
	return out
}

func permsToStrings(perms []perm.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	return out
}
