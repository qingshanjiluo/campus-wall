import { storeToRefs } from 'pinia'

// Frontend mirror of the OAuth role contract (docs/oauth/11-roles.md — Tier A;
// apps/api/pkg/role is the authoritative binding). Roles are an unordered SET of
// claim strings on the user store. Two facts drive everything:
//
//   - `user` is the implicit default and NEVER appears — a plain logged-in user
//     has roles = []. Detect login by the user id / session, NEVER by role
//     membership (there is deliberately no `user` constant).
//   - Authority is two orthogonal axes: a management hierarchy
//     (moderator ⊂ admin ⊂ ren, inclusive) and the orthogonal `creator`
//     direct-publish capability.
//
// Frontend gating is UX ONLY — the backend (pkg/role) is the real boundary.
// Components branch on a CAPABILITY here, never on a role number.

// The exact claim strings (contract §1) — for the rare direct check / badge.
export const ROLE = {
  creator: 'creator',
  moderator: 'moderator',
  admin: 'admin',
  ren: 'ren'
} as const

export const useRole = () => {
  const { roles } = storeToRefs(usePersistUserStore())

  // Management axis, inclusive: admin & ren subsume moderator; ren subsumes admin.
  const canModerate = computed(() =>
    roles.value.some(
      (r) => r === ROLE.moderator || r === ROLE.admin || r === ROLE.ren
    )
  )
  const canAdminister = computed(() =>
    roles.value.some((r) => r === ROLE.admin || r === ROLE.ren)
  )
  // Orthogonal trusted-publisher capability — no moderation/admin power.
  const isCreator = computed(() => roles.value.includes(ROLE.creator))

  return { canModerate, canAdminister, isCreator }
}
