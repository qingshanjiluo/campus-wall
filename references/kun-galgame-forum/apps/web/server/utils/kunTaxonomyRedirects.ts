// Legacy taxonomy URL → current canonical URL (A2-3 / doc 106 R1, doc 146).
//
// Two generations of URL are retired, and both redirect here in ONE hop:
//
//   gen 1  /galgame-tag/{wikiId}        wiki id space
//   gen 2  /galgame-tag/c/{catalogId}   catalog id space, `/c/` discriminator
//   now    /galgame/tag/{catalogId}     catalog id space, bare
//
// Why gen 2 needed the `/c/` segment: in the `/galgame-tag/` space a bare number
// was a WIKI id, and the two id spaces OVERLAP densely — 718 of the 1,530 mapped
// wiki tag ids are themselves live catalog tag ids, and 442 of the 1,507 retired
// ones are too — so `/galgame-tag/{n}` could not be resolved to one meaning or
// the other. The discriminator made the question decidable forever: a bare
// number is ALWAYS a wiki id, `/c/{n}` is ALWAYS a catalog id, no matter how
// either range grows.
//
// Why the current space does not need it: `/galgame/tag/` never served wiki ids
// at all. The ambiguity the segment guarded against cannot arise there, so it is
// pure baggage and the catalog id is carried bare.
//
// The maps are frozen exports of the production registry (infra
// refs/proj/132-artifacts + 127-artifacts) and are vendored rather than fetched:
// they describe a one-time historical migration, so they can never drift, and a
// redirect must not depend on a live service being reachable.

// The path builders moved to shared/ (doc 148) — the detail PAGES need them
// too, to hop a merged catalog id. Imported explicitly rather than leaned on
// auto-import so this module stays runnable from a plain vitest context.
import { taxonomyDetailPath } from '../../shared/utils/kunTaxonomyPaths'
import tagRedirects from '../data/wiki-tag-redirects.json'
import engineRedirects from '../data/wiki-engine-redirects.json'
import tagGone from '../data/wiki-tag-gone.json'

const tagMap = tagRedirects as Record<string, number>
const engineMap = engineRedirects as Record<string, number>
// The wiki tags with no canonical counterpart. They are 410 rather than 404:
// the URL was valid and the resource is permanently gone, which is what tells a
// crawler to drop it instead of re-checking forever.
const goneTags = new Set<number>(tagGone as number[])

export type LegacyResolution =
  | { kind: 'redirect'; to: string }
  | { kind: 'gone' }
  | { kind: 'missing' }

/** Parse a legacy path segment as a positive wiki id. */
export const parseWikiId = (raw: string | undefined): number | null => {
  if (!raw) return null
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : null
}

/** Resolve a legacy tag id: 301 when mapped, 410 when retired, else 404. */
export const resolveLegacyTag = (wikiId: number): LegacyResolution => {
  const target = tagMap[String(wikiId)]
  if (target) return { kind: 'redirect', to: taxonomyDetailPath('tag', target) }
  if (goneTags.has(wikiId)) return { kind: 'gone' }
  return { kind: 'missing' }
}

/**
 * Resolve a legacy engine id. The mapping is NOT the identity — 52 of the 189
 * engines landed on a different catalog id — so it has to be looked up rather
 * than assumed.
 */
export const resolveLegacyEngine = (wikiId: number): LegacyResolution => {
  const target = engineMap[String(wikiId)]
  if (target)
    return { kind: 'redirect', to: taxonomyDetailPath('engine', target) }
  return { kind: 'missing' }
}

// The gen-2 `/galgame-{family}` space. Its index and its `/c/{id}` detail form
// both carry straight over: the id space is unchanged and only the spelling of
// the path moved, so this is a pure rename with no lookup involved.
const RENAMED_INDEX_RE = /^\/galgame-(tag|official|engine)$/
const RENAMED_DETAIL_RE = /^\/galgame-(tag|official|engine)\/c\/(\d+)$/

/**
 * Resolve a retired `/galgame-{family}` URL to its current form, or null when
 * the path is not one of them. Catalog ids pass through untouched.
 */
export const resolveRenamedTaxonomyPath = (path: string): string | null => {
  const index = path.match(RENAMED_INDEX_RE)
  if (index) return taxonomyIndexPath(index[1] as TaxonomyFamily)

  const detail = path.match(RENAMED_DETAIL_RE)
  if (detail) {
    const catalogId = Number(detail[2])
    if (!Number.isInteger(catalogId) || catalogId <= 0) return null
    return taxonomyDetailPath(detail[1] as TaxonomyFamily, catalogId)
  }

  return null
}
