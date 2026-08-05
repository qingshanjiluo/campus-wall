// Background-revalidate the current user's display fields when they return to
// the tab or the network comes back online, so a profile change made elsewhere
// (the OAuth site / moyu — shared identity) appears without a logout. The
// full-page-load trigger lives in validate-session.client.ts; this adds only
// the focus / online triggers. See useRefreshMe for the throttle + safety
// rationale (it dedupes, so wiring both hooks can't double-fetch).
//
// visibilitychange (not `focus`) mirrors React Query v5 — it ignores trivial
// focus churn (clicking the window chrome) and fires only on a real foreground.
// runWithContext is mandatory: a bare DOM-event callback runs outside any Nuxt
// context, so kunFetch / the Pinia store inside refreshMe would throw without it.
export default defineNuxtPlugin((nuxtApp) => {
  // Safe at init — useRefreshMe touches neither the store nor Nuxt context until
  // refreshMe() actually runs (inside runWithContext below).
  const { refreshMe } = useRefreshMe()

  const onVisible = () => {
    if (document.visibilityState === 'visible') {
      nuxtApp.runWithContext(() => refreshMe())
    }
  }
  const onOnline = () => nuxtApp.runWithContext(() => refreshMe())

  document.addEventListener('visibilitychange', onVisible)
  window.addEventListener('online', onOnline)
})
