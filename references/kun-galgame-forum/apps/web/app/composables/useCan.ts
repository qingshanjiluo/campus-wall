import type { ComputedRef } from 'vue'
import { storeToRefs } from 'pinia'

// Frontend mirror of apps/api/pkg/perm — the pure-forum permission bundle.
//
// Two sources of truth, in priority order:
//   1. RUNTIME: /perm/mine ships the CURRENT user's EFFECTIVE permission list
//      (the role layer plus admin-set personal grant/revoke overrides, folded
//      in). The perm-mine plugin fetches it once per app load into
//      useState('kun-perm-mine'); when present, a plain membership test on it is
//      authoritative here (it already reflects the caller's actual capabilities,
//      including any per-user override on a roleless account).
//   2. FALLBACK: the static role table below is the COMPILED BASELINE — used
//      before the fetch resolves and if it fails (offline / logged-out). It maps
//      roles only, so it can't see personal overrides; it is hand-maintained and
//      MUST stay in lockstep with pkg/perm, byte-for-byte (the string values are
//      the wire contract).
//
// Frontend gating is UX ONLY — the backend (pkg/perm, with the same overrides
// applied) is the real boundary; components branch on a named CAPABILITY here,
// never on a role tier.
//
// Scope: exactly the 43 PURE-FORUM permissions. The 9 INFRA-PROXY permissions
// (galgame edit-proposal review, taxonomy admin, trust moderation inbox,
// galgame submission review, …) deliberately live OUTSIDE this table — their
// source of truth is infra, not this service. Those UI spots keep useRole()
// (or an API-provided capability projection, e.g. `can_review` / `can_decide`)
// with a mirror comment naming the infra permission they stand in for.
//
// Thresholds: `moderator` holds the 41 mod keys; `admin` and `ren` hold all 43
// (the 41 mod keys + the 2 admin-only keys). `user` and `creator` hold NO forum
// permissions. The admin bundle is built FROM the moderator list plus the two
// admin-only keys, so `moderator ⊂ admin` holds by construction.

// The 41 permissions a moderator holds (admin & ren inherit these).
const MODERATOR_PERMISSIONS = [
  'topic.edit_any',
  'topic.hide',
  'topic.set_best_answer',
  'reply.delete_any',
  'reply.pin',
  'comment.topic.delete',
  'comment.galgame.edit',
  'comment.galgame.delete',
  'comment.rating.edit',
  'comment.rating.delete',
  'comment.website.edit',
  'comment.website.delete',
  'comment.toolset.edit',
  'comment.toolset.delete',
  'comment.resource.edit',
  'comment.resource.delete',
  'comment.quiz.edit',
  'comment.quiz.delete',
  'poll.create_any',
  'poll.edit_any',
  'poll.delete_any',
  'poll.view_restricted',
  'galgame.ban_resource_publish',
  'quiz.edit_any',
  'quiz.delete_any',
  'resource.edit_any',
  'resource.delete_any',
  'rating.delete_any',
  'toolset.edit_any',
  'toolset.delete_any',
  'toolset.resource.edit_any',
  'toolset.resource.delete_any',
  'toolset.upload_bypass',
  'doc.create',
  'doc.edit',
  'doc.delete',
  'website.create',
  'website.edit',
  'website.delete',
  'friend_link.create',
  'friend_link.edit',
  'friend_link.delete',
  'update_log.create',
  'update_log.edit',
  'update_log.delete'
] as const

// The 2 permissions only admin & ren hold, on top of the moderator set.
const ADMIN_ONLY_PERMISSIONS = [
  'admin.dashboard',
  'user.purge_content'
] as const

// admin === ren: the full 43. Built from the moderator list so containment is
// structural, not a copy that could drift.
const ADMIN_PERMISSIONS = [
  ...MODERATOR_PERMISSIONS,
  ...ADMIN_ONLY_PERMISSIONS
] as const

// The string-literal union of every pure-forum permission key.
export type ForumPermission =
  | (typeof MODERATOR_PERMISSIONS)[number]
  | (typeof ADMIN_ONLY_PERMISSIONS)[number]

// Role → granted-permission bundles. Keyed by the raw OAuth role claim; roles
// not present here (`user`, `creator`) grant nothing. `has()` lookups keep
// useCan O(1) regardless of table size.
const ROLE_PERMISSIONS: Record<string, ReadonlySet<ForumPermission>> = {
  moderator: new Set(MODERATOR_PERMISSIONS),
  admin: new Set(ADMIN_PERMISSIONS),
  ren: new Set(ADMIN_PERMISSIONS)
}

// Reactive, UX-only capability check. Reads the fetched personal list when it
// has landed in useState, otherwise the static role table (still reactive to
// role changes via storeToRefs, so a role-only fallback tracks login/logout).
export const useCan = (permission: ForumPermission): ComputedRef<boolean> => {
  const { roles } = storeToRefs(usePersistUserStore())
  // The perm-mine plugin populates this with the current user's EFFECTIVE
  // permission list; null until fetched / on failure.
  const mine = useState<string[] | null>('kun-perm-mine', () => null)

  return computed(() => {
    const list = mine.value
    if (list) {
      // Runtime truth: /perm/mine already folds in the role layer AND the
      // caller's personal grant/revoke deltas, so a plain membership test is
      // exact — including a permission granted to an otherwise roleless user.
      return list.includes(permission)
    }
    // Compiled-baseline fallback (pre-fetch / offline / logged-out): the static
    // role table. It can't see personal overrides, but this is UX-only gating
    // and the backend (with overrides applied) is the real boundary.
    return roles.value.some(
      (role) => ROLE_PERMISSIONS[role]?.has(permission) ?? false
    )
  })
}

// Reactive predicate over the CURRENT user's OWN effective permission set — the
// shared possession helper for the admin permission editors (Matrix / UserPanel).
// It resolves each key with the SAME priority as useCan (runtime /perm/mine list
// when landed, else the static role table), so an operator may only add/remove an
// override for a key they themselves hold. UX only; the backend possession guard
// (perm.EffectiveForUser) is the real boundary.
export const useMyPermissions = (): ComputedRef<
  (permission: string) => boolean
> => {
  const { roles } = storeToRefs(usePersistUserStore())
  const mine = useState<string[] | null>('kun-perm-mine', () => null)

  return computed(() => {
    const list = mine.value
    if (list) {
      return (permission: string) => list.includes(permission)
    }
    return (permission: string) =>
      roles.value.some(
        (role) =>
          ROLE_PERMISSIONS[role]?.has(permission as ForumPermission) ?? false
      )
  })
}
