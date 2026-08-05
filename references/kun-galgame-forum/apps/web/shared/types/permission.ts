// Wire types for the permission-first authorization admin surface.
//
// The runtime truth is a DELTA model: a role's effective bundle is its compiled
// code baseline, adjusted by admin-set OVERRIDE rows (grant / revoke) persisted
// in Postgres. An override row exists ONLY where the role deviates from its
// baseline, so `effective = (baseline ∪ grants) − revokes`.
//
// `permission` / `catalog` values are the pure-forum ForumPermission keys (see
// app/composables/useCan.ts); they are typed as plain strings here because this
// is untransformed wire data — the constants layer is where the ForumPermission
// vocabulary is enforced.

export type KunRolePermEffect = 'grant' | 'revoke'

// A single deviation from the compiled baseline for one role.
export interface KunRolePermOverride {
  permission: string
  effect: KunRolePermEffect
  updated_by?: number
  updated_at?: string
}

// One role's row in the matrix. `locked` marks the `ren` (站长) safety anchor:
// it always holds the full catalogue and is never adjustable.
export interface KunRolePermState {
  baseline: string[]
  overrides: KunRolePermOverride[]
  effective: string[]
  locked?: boolean
}

// GET /admin/role-permissions and every PUT /admin/role-permissions/:role
// response share this shape. `roles` is keyed by the raw role claim
// (creator / moderator / admin / ren); `user` never appears (implicit claim).
export interface KunRolePermMatrix {
  catalog: string[]
  roles: Record<string, KunRolePermState>
}

// GET /perm/mine (any logged-in user): the CURRENT user's EFFECTIVE pure-forum
// permission list — the role layer plus admin-set personal grant/revoke deltas,
// already folded in. Consumed by the perm-mine plugin; the runtime truth useCan
// reads. A plain list because the client only ever membership-tests it.
export interface KunPermMine {
  permissions: string[]
}

// GET /admin/user-permissions/:uid and every PUT response share this shape.
// `role_effective` is the role-derived reference set (the deviation baseline in
// the per-user panel); `effective` folds in this user's personal grant/revoke
// overrides on top of it.
export interface KunUserPermState {
  user_id: number
  roles: string[]
  role_effective: string[]
  overrides: KunRolePermOverride[]
  effective: string[]
}

// One row of the permission audit log. `before` / `after` are the OVERRIDE sets
// (relative to a role's compiled baseline, or a user's role_effective) captured
// immediately before and after the change — the FE diffs them client-side to
// render the delta. `subject` is a role claim (subject_kind 'role') or a uid
// string (subject_kind 'user').
export interface KunPermAuditEntry {
  id: number
  operator_id: number
  subject_kind: 'role' | 'user'
  subject: string
  action: 'replace' | 'reset'
  before: KunRolePermOverride[]
  after: KunRolePermOverride[]
  created_at: string
}

// GET /admin/permission-audit?page=&limit= — newest first, page/limit paging.
// `users` resolves operator / user-subject ids to display chips (uid → brief).
export interface KunPermAuditPage {
  total: number
  items: KunPermAuditEntry[]
  users: Record<string, KunUser>
}
