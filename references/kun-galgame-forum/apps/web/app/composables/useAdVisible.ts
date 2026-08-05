import { storeToRefs } from 'pinia'

// Whether the AIFY promo ad slots should be shown to the current visitor.
//
// Policy: ads are shown to anonymous visitors and plain `user` accounts, and
// WAIVED for anyone holding a role beyond `user` (版主 / 管理员 / 创作者 / …) —
// an ad-free perk for privileged / special identities. Per the OAuth role
// contract `user` is implicit and never appears, so "holds a non-user role" is
// exactly "the role set is non-empty".
//
//   anonymous       roles = []            → empty set → show
//   regular user    roles = []            → empty set → show
//   privileged      roles = ['admin'] (or any claim) → non-empty → hide
//
// Reactive (storeToRefs): the slot toggles correctly across login / logout /
// role change without a remount.
export const useAdVisible = () => {
  const { roles } = storeToRefs(usePersistUserStore())
  return computed(() => roles.value.length === 0)
}
