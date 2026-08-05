// Sync a tab selection to a URL query param so the active tab survives
// back/forward, refresh, and is shareable — the "URL as the source of truth for
// navigational state" pattern. Returns a writable computed for v-model.
//
// - The default tab is omitted from the URL (clean `/` instead of `/?tab=all`).
// - Uses router.replace, not push: switching tabs shouldn't pile up history
//   entries — pressing back should leave the page, not step through prior tabs.
//
// Pair with definePageMeta({ keepalive: true }) on the page to ALSO preserve
// scroll + already-loaded list items across a visit-and-return.
export const useTabQuery = (defaultValue: string, key = 'tab') => {
  const route = useRoute()
  const router = useRouter()

  return computed<string>({
    get() {
      const v = route.query[key]
      const val = Array.isArray(v) ? v[0] : v
      return val || defaultValue
    },
    set(val) {
      const query = { ...route.query }
      if (val === defaultValue) {
        delete query[key]
      } else {
        query[key] = val
      }
      router.replace({ query })
    }
  })
}
