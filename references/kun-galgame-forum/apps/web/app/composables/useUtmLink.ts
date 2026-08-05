// Appends `utm_source=<current domain>` to an outbound URL so partner sites
// (友情链接) and the website-wiki targets can attribute click-throughs back to
// whichever kungal domain the visitor is actually on.
//
// `useRequestURL()` is read ONCE here (setup / SSR-safe — host = the current
// request's domain on the server, window.location's on the client). The
// returned helper is pure, so it's safe to call inside a template / v-for
// without re-entering a Nuxt composable per call.
export const useUtmLink = () => {
  const host = useRequestURL().host

  return (rawUrl: string): string => {
    if (!rawUrl) {
      return rawUrl
    }
    try {
      // Bare domains (no scheme) are coerced to https so they parse + stay
      // external; anything unparseable is returned untouched.
      const url = new URL(
        /^https?:\/\//i.test(rawUrl) ? rawUrl : `https://${rawUrl}`
      )
      url.searchParams.set('utm_source', host)
      return url.toString()
    } catch {
      return rawUrl
    }
  }
}
