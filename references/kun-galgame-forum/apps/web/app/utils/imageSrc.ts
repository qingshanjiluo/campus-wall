// Resolve a content-addressed image reference (a bare 64-hex hash or a
// /image/<hash> token) to an ABSOLUTE CDN URL, mirroring the image_service /
// server-middleware layout: {base}/<aa>/<bb>/<hash>[_<variant>].webp.
//
// Why absolute (not the relative /image/<hash> token): a relative token routed
// through @nuxt/image's <NuxtImg> becomes /_ipx/_/image/<hash>, which IPX can't
// resolve → 404; and even a plain <img> on the token pays a /image 302 hop on
// the app server. An absolute CDN URL skips both — @nuxt/image passes absolute
// URLs through untouched, and the browser hits the CDN directly. Same fast path
// the SSR markdown renderer + galgame `cdn_url` already use.

// PURE builder — no Nuxt context, unit-testable.
export const imageHashUrl = (
  base: string,
  hash: string,
  variant?: string
): string => {
  const b = (base || '').replace(/\/+$/, '')
  const file = variant ? `${hash}_${variant}.webp` : `${hash}.webp`
  return `${b}/${hash.slice(0, 2)}/${hash.slice(2, 4)}/${file}`
}

// The configured public CDN base (runtimeConfig.public.imageCdnBase). Falls back
// to the literal default (mirrors nuxt.config) when there's no Nuxt context —
// useRuntimeConfig throws outside the app (e.g. a pure unit test), and these
// resolvers are exercised in happy-dom specs.
export const imageCdnBase = (): string => {
  try {
    const v = useRuntimeConfig().public.imageCdnBase as string | undefined
    if (v) return v.replace(/\/+$/, '')
  } catch {
    // no Nuxt app — fall through to the default
  }
  return 'https://image.kungal.iloveren.link'
}

// Resolve a /image/<hash> content token (or a bare hash) to an absolute CDN URL.
// Anything already absolute (http…) or empty is returned unchanged.
export const imageTokenUrl = (tokenOrHash: string): string => {
  if (!tokenOrHash || tokenOrHash.startsWith('http')) return tokenOrHash
  const hash = tokenOrHash.startsWith('/image/')
    ? tokenOrHash.slice('/image/'.length)
    : tokenOrHash
  return imageHashUrl(imageCdnBase(), hash)
}
