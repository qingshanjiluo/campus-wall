// 302 fallback for the domain-independent content image token `/image/<hash>`.
//
// Implemented as global middleware (not a server route) on purpose: `public/image/`
// exists (mascot assets like /image/kohaku.webp), and a `server/routes/image/` dir
// collides with it — Nitro silently skips the route. Middleware runs before
// public-asset serving and only claims paths that match the strict token shape, so
// real /image/*.webp assets still fall through untouched.
//
// Server-rendered content already gets the token resolved to an absolute CDN URL by
// the Go markdown renderer (the fast path, no redirect). This covers everything that
// renders the RAW token client-side or out-of-band — the milkdown editor preview,
// RSS readers, email, old caches. The only "hash to domain" mapping is the single
// `imageCdnBase` config, so a CDN/domain change is one env flip with zero content
// rewrite (image_service contract, docs/image_service/06-integration-guide.md).
//
// CDN layout mirrors imageclient.MainURL/VariantURL exactly:
//   main     {base}/<aa>/<bb>/<hash>.webp
//   variant  {base}/<aa>/<bb>/<hash>_<variant>.webp     (aa=hash[:2], bb=hash[2:4])
const TOKEN_RE = /^\/image\/([0-9a-f]{64})(?:_([a-z0-9]+))?$/

export default defineEventHandler((event) => {
  if (event.method !== 'GET') return
  const path = event.path.split('?')[0]!
  const m = path.match(TOKEN_RE)
  if (!m) return // not a token → fall through (public asset / page / 404)

  const hash = m[1]!
  const variant = m[2]
  const base = (useRuntimeConfig(event).imageCdnBase || '').replace(/\/+$/, '')
  const file = variant ? `${hash}_${variant}.webp` : `${hash}.webp`
  return sendRedirect(
    event,
    `${base}/${hash.slice(0, 2)}/${hash.slice(2, 4)}/${file}`,
    302
  )
})
