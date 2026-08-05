import { imageCdnBase, imageHashUrl } from './imageSrc'

// Resolve a cover/screenshot row to a displayable image URL.
//
// Prefer the server-injected `cdn_url` (kungal's rewriteBanners walker) when
// present. Otherwise resolve `image_hash` to an ABSOLUTE CDN URL — so a row that
// arrived WITHOUT `cdn_url` (e.g. a re-edit hydration that didn't pass through
// the walker) still shows the picture. It must be absolute, NOT the relative
// /image/<hash> token: routed through KunImage → @nuxt/image IPX, a relative
// token becomes /_ipx/_/image/<hash> → 404 (the old token fallback was silently
// broken in exactly the case it was meant to fix). See utils/imageSrc.ts.
export const galgameImageSrc = (row: {
  cdn_url?: string
  image_hash: string
}): string => row.cdn_url || imageHashUrl(imageCdnBase(), row.image_hash)
