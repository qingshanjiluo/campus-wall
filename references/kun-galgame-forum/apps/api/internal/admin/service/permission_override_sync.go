package service

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/internal/admin/model"
	"kun-galgame-api/pkg/perm"
)

// roleRowLister / userRowLister are the narrow persistence seams the sync needs —
// just the full scan each override layer feeds from. The concrete repositories
// satisfy them; tests can supply fakes.
type roleRowLister interface {
	ListAll(ctx context.Context) ([]model.RolePermissionOverride, error)
}

type userRowLister interface {
	ListAll(ctx context.Context) ([]model.UserPermissionOverride, error)
}

// PermissionOverrideSync owns the SINGLE Load path that refreshes BOTH runtime
// override layers in pkg/perm — role deltas → SetOverrides, user deltas →
// SetUserOverrides. Boot, the background refresher, and every write-through (role
// OR user replace) all go through it, so the in-memory effective table converges
// uniformly from one code path. It satisfies the reloader seam both editors use.
type PermissionOverrideSync struct {
	roleStore roleRowLister
	userStore userRowLister
}

func NewPermissionOverrideSync(roleStore roleRowLister, userStore userRowLister) *PermissionOverrideSync {
	return &PermissionOverrideSync{roleStore: roleStore, userStore: userStore}
}

// Load reads BOTH override tables and pushes them into pkg/perm. It is
// fail-atomic: if EITHER read errors it returns the error WITHOUT touching
// pkg/perm, so the last-known effective sets keep serving (the override tables
// being unreachable must never fail requests). The caller (boot / refresher /
// write-through) logs a warning on error.
func (s *PermissionOverrideSync) Load(ctx context.Context) error {
	roleRows, err := s.roleStore.ListAll(ctx)
	if err != nil {
		return err
	}
	userRows, err := s.userStore.ListAll(ctx)
	if err != nil {
		return err
	}
	perm.SetOverrides(rowsToOverrideMap(roleRows))
	perm.SetUserOverrides(userRowsToOverrideMap(userRows))
	return nil
}

// StartRefresher launches a background goroutine that re-Loads BOTH layers every
// interval, so manual psql edits (and any future second instance) converge
// without a restart. A single goroutine drives it, so runs never overlap. A Load
// failure warns and keeps the last-known effective sets — it never crashes.
// Returns a stop function.
func (s *PermissionOverrideSync) StartRefresher(interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := s.Load(context.Background()); err != nil {
					slog.Warn("权限覆盖刷新失败, 继续沿用上一次的有效权限", "error", err)
				}
			}
		}
	}()
	return func() { close(done) }
}

// userRowsToOverrideMap groups per-user override rows into the per-uid map
// pkg/perm.SetUserOverrides consumes (which filters unknown keys / non-positive
// uids defensively).
func userRowsToOverrideMap(rows []model.UserPermissionOverride) map[int][]perm.Override {
	out := make(map[int][]perm.Override)
	for _, r := range rows {
		out[r.UserID] = append(out[r.UserID], perm.Override{
			Permission: perm.Permission(r.Permission),
			Effect:     r.Effect,
		})
	}
	return out
}
