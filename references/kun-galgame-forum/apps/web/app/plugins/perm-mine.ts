// Fetch the CURRENT user's EFFECTIVE pure-forum permission list (the role layer
// plus admin-set personal grant/revoke deltas) once per app load, into
// useState('kun-perm-mine'). useCan reads it as its runtime truth, falling back
// to the static compiled role table until / unless this resolves.
//
// Universal (not .client) on purpose: fetching during SSR populates the state
// before first paint and transfers it via the Nuxt payload, so a user's SSR and
// hydrated renders agree — no capability flash, no hydration mismatch. The guard
// below means the client re-run is a no-op once the payload is in.
//
// Fetched for ANY logged-in user (roles are irrelevant now): a personal grant
// can land on a ROLELESS plain user, so the old "skip when roles is empty" guard
// would wrongly hide a legitimately-granted capability. Only anonymous visitors
// (no session id) skip — they hold nothing whether we fetch or not.
export default defineNuxtPlugin(async () => {
  const mine = useState<string[] | null>('kun-perm-mine', () => null)

  // Already populated (SSR payload transferred to the client) — don't refetch.
  if (mine.value) return

  // The persisted store id is cookie-backed, so it's available on SSR too.
  const { id } = usePersistUserStore()
  if (!id) return

  // Failure leaves the state null (useCan falls back to the static baseline).
  const data = await kunFetch<KunPermMine>('/perm/mine')
  if (data) {
    mine.value = data.permissions
  }
})
