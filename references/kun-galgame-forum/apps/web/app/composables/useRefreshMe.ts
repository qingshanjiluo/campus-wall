// SWR revalidation of the current user's display fields (name / avatar /
// roles). The persisted KUNGalgameUser store renders these INSTANTLY (the stale
// layer, survives SSR with no flash); this refreshes them in the background at
// smart moments — full page load, tab refocus, network back online — so a
// profile change made elsewhere (the OAuth identity site, or moyu — identity is
// shared across all downstreams, see CLAUDE.md C1/C2) shows up WITHOUT a
// logout+login.
//
// Why /auth/me: /user/status is hit on every load but returns only moemoepoint /
// check-in / message flags — NOT name/avatar/roles. /auth/me is the only endpoint
// that returns the display fields fresh, yet the app otherwise calls it just
// once, at the OAuth callback. This closes that gap.
//
// Client-only. Throttled (60s) + in-flight dedupe so refocusing a pile of tabs
// can't hammer /auth/me. The module-scoped state below is written ONLY under
// import.meta.client (refreshMe returns early on the server), so it never leaks
// across SSR requests. The throttle window resets every full page load (a fresh
// JS context zeroes lastFetchedAt), so a hard refresh always revalidates.

interface MeResponse {
  id: number
  sub: string
  name: string
  avatar: string
  roles: string[]
  moemoepoint: number
  bio: string
}

const STALE_MS = 60_000
let lastFetchedAt = 0
let inFlight: Promise<void> | null = null

export const useRefreshMe = () => {
  const refreshMe = (): Promise<void> => {
    if (!import.meta.client) return Promise.resolve()

    const userStore = usePersistUserStore()
    // Nothing to revalidate when logged out.
    if (!userStore.id) return Promise.resolve()
    // Coalesce concurrent callers (e.g. the load hook + a near-simultaneous
    // refocus) onto the single in-flight request.
    if (inFlight) return inFlight
    if (Date.now() - lastFetchedAt < STALE_MS) return Promise.resolve()

    // Capture the config synchronously (inside the caller's Nuxt context) so the
    // async continuation doesn't depend on context surviving the await.
    const config = useRuntimeConfig()

    inFlight = (async () => {
      // Raw $fetch, NOT kunFetch, ON PURPOSE: this is a SILENT background refresh
      // the user never initiated (it fires on tab-refocus / reconnect). kunFetch
      // toasts every non-205 error (handleApiError → useMessage, and the catch's
      // "网络请求失败"), so routing /auth/me through it would pop a user-facing
      // error toast on every refocus during an OAuth hiccup. We fetch raw, SWALLOW
      // failures (keep the cached identity, no toast, no logout), and let the
      // sibling /user/status — still via kunFetch, on full load — own dead-session
      // detection. Client-only here, so the client baseURL is all we need.
      const resp = await $fetch<{ code: number; data?: MeResponse }>(
        `${config.public.apiBaseUrl}/api/auth/me`,
        { credentials: 'include' }
      ).catch(() => null)
      const me = resp?.code === 0 ? resp.data : null

      // Merge only when /auth/me is still the SAME identity now in the store. id
      // is stable per user (C1/C2 — never renumbered), so a mismatch means the
      // session changed underneath us between request and response: a logout
      // mid-flight (id→0, via the sibling /user/status path) or an account switch
      // in another tab (the session cookie is shared). Writing display fields
      // then would split-brain the store — a new avatar/name on an old id.
      // Skipping keeps it coherent; identity is reconciled wholesale by the
      // login/switch callback, not here.
      if (me?.name && me.id === userStore.id) {
        userStore.setProfileInfo({
          name: me.name,
          avatar: me.avatar,
          roles: me.roles ?? []
        })
      }
      // Stamp even on failure so a persistently-failing /auth/me can't be retried
      // on every refocus (the throttle is the storm guard); the next full page
      // load resets the window regardless.
      lastFetchedAt = Date.now()
    })().finally(() => {
      inFlight = null
    })

    return inFlight
  }

  return { refreshMe }
}
