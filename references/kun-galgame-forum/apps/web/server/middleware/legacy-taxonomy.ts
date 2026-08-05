// Retired taxonomy URL shells (A2-3 / doc 106 R1, doc 146).
//
// Two generations of taxonomy URL are retired, and EVERY one of them lands on
// its final form in a single hop:
//
//   /galgame-tag/{wikiId}          →  /galgame/tag/{catalogTagId}
//   /galgame-tag/c/{catalogId}     →  /galgame/tag/{catalogId}
//   /galgame-tag                   →  /galgame/tag
//   …and the same three shapes for official / engine.
//
// The one-hop rule is the point. The obvious implementation — leave the gen-1
// shells pointing at gen 2 and let the new gen-2 rule forward them again — costs
// a redirect chain on exactly the URLs search engines already hold, which is
// where link equity is lost and crawlers start giving up. So gen-1 resolution
// targets the CURRENT form directly; both generations are resolved here, in one
// pass, never by bouncing one off the other.
//
// Why gen 2 ever had a `/c/` segment: in the `/galgame-tag/` space a bare number
// was a WIKI id, and the two id spaces OVERLAP densely — 718 of the 1,530 mapped
// wiki tag ids are themselves live catalog tag ids, and 442 of the 1,507 retired
// ones are too — so a single path serving both meanings would silently render
// the wrong entity for whichever one lost. The discriminator made it decidable
// forever. `/galgame/tag/` never served wiki ids, so it needs no such segment.
//
// This runs as global middleware rather than as shell PAGES so the redirect is
// issued before any rendering — a crawler gets a clean 301 with no HTML body,
// and there is no client-side hydration for a URL that is never meant to render.
// Paths that match nothing fall straight through, including the live pages.
//
// Three outcomes, and the difference between the last two is what a crawler
// acts on:
//   301  the entity has a canonical counterpart
//   410  it had none (the 1,507 retired tags) — permanently gone, so a crawler
//        drops it instead of re-checking forever
//   404  never a valid id in the first place
import {
  parseWikiId,
  resolveLegacyTag,
  resolveLegacyEngine,
  resolveRenamedTaxonomyPath
} from '../utils/kunTaxonomyRedirects'
import { taxonomyDetailPath } from '../../shared/utils/kunTaxonomyPaths'

// Strict shape: exactly one numeric segment under one of the three families.
// `/c/…` cannot match (it has a non-numeric segment), and neither can the index
// pages or any deeper path.
const LEGACY_RE = /^\/galgame-(tag|official|engine)\/(\d+)$/

export default defineEventHandler(async (event) => {
  if (event.method !== 'GET') return
  const [path, query] = event.path.split('?') as [string, string?]

  // Query survives the hop: the index pages carry filter + pagination state, and
  // dropping it would silently reset a shared link to page 1.
  const withQuery = (to: string) => (query ? `${to}?${query}` : to)

  // gen 2 → now. Pure rename, no lookup: the id space is identical.
  const renamed = resolveRenamedTaxonomyPath(path)
  if (renamed) return sendRedirect(event, withQuery(renamed), 301)

  // gen 1 → now, resolved straight to the current form (never via gen 2).
  const m = path.match(LEGACY_RE)
  if (!m) return

  const family = m[1]!
  const wikiId = parseWikiId(m[2])
  if (wikiId === null) return // not a usable id → let the router 404 it

  if (family === 'tag') {
    const res = resolveLegacyTag(wikiId)
    if (res.kind === 'redirect') return sendRedirect(event, res.to, 301)
    if (res.kind === 'gone') {
      throw createError({
        statusCode: 410,
        statusMessage: '该标签已在词表迁移中退役'
      })
    }
    return // unknown wiki id → fall through to the router's 404
  }

  if (family === 'engine') {
    // NOT the identity mapping: 52 of the 189 engines landed on a different
    // catalog id, so this has to be looked up rather than assumed.
    const res = resolveLegacyEngine(wikiId)
    if (res.kind === 'redirect') return sendRedirect(event, res.to, 301)
    return
  }

  // Makers resolve at runtime through the registry's external-ref index (A2-0
  // registered 100% of them), so future merges keep redirecting correctly —
  // something a frozen map could not do. An unreachable API falls through to a
  // 404 rather than erroring the request: a missing redirect is recoverable,
  // a 500 on a crawled URL is not.
  //
  // The API path below is the BACKEND's own endpoint namespace, which is
  // unrelated to the frontend route rename and deliberately untouched by it.
  const id = await $fetch<{ data?: { id?: number } }>(
    `${useRuntimeConfig().apiBaseUrl}/api/galgame-official/legacy/${wikiId}`,
    { headers: { accept: 'application/json' } }
  )
    .then((r) => r?.data?.id)
    .catch(() => undefined)

  if (id) return sendRedirect(event, taxonomyDetailPath('official', id), 301)
})
