// Taxonomy redirect contract (doc 146).
//
// Two generations of taxonomy URL are retired and both must land on the CURRENT
// form in a single hop. The one-hop rule is the whole point of these tests: the
// cheap implementation — leaving gen-1 pointing at gen-2 and letting the gen-2
// rule forward it again — is invisible in a unit test that only checks "did it
// redirect", which is why the assertions below check the SHAPE of every target
// rather than just its presence.
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect } from 'vitest'
import {
  parseWikiId,
  resolveLegacyTag,
  resolveLegacyEngine,
  resolveRenamedTaxonomyPath
} from '../server/utils/kunTaxonomyRedirects'
// The path builders moved to shared/ (doc 148): the detail pages need them too,
// to hop a merged catalog id, and a page cannot import from server/.
import {
  TAXONOMY_FAMILIES,
  taxonomyIndexPath,
  taxonomyDetailPath
} from '../shared/utils/kunTaxonomyPaths'
import type { TaxonomyFamily } from '../shared/utils/kunTaxonomyPaths'
import tagRedirects from '../server/data/wiki-tag-redirects.json'
import engineRedirects from '../server/data/wiki-engine-redirects.json'
import tagGone from '../server/data/wiki-tag-gone.json'

// A final-form detail URL: the `/galgame/` namespace, a bare catalog id, and
// none of the retired spellings anywhere in it.
const FINAL_DETAIL = /^\/galgame\/(tag|official|engine)\/\d+$/
const isFinalForm = (path: string) =>
  FINAL_DETAIL.test(path) &&
  !path.includes('/galgame-') &&
  !path.includes('/c/')

describe('taxonomy path builders', () => {
  it('builds the current index path per family', () => {
    expect(taxonomyIndexPath('tag')).toBe('/galgame/tag')
    expect(taxonomyIndexPath('official')).toBe('/galgame/official')
    expect(taxonomyIndexPath('engine')).toBe('/galgame/engine')
  })

  it('builds a bare catalog-id detail path with no discriminator segment', () => {
    expect(taxonomyDetailPath('tag', 55)).toBe('/galgame/tag/55')
    expect(taxonomyDetailPath('official', 9)).toBe('/galgame/official/9')
    expect(taxonomyDetailPath('engine', 138)).toBe('/galgame/engine/138')
  })
})

describe('gen-2 `/galgame-{family}` → current, one hop', () => {
  it.each([...TAXONOMY_FAMILIES])('redirects the %s index', (family) => {
    expect(resolveRenamedTaxonomyPath(`/galgame-${family}`)).toBe(
      `/galgame/${family}`
    )
  })

  it.each([...TAXONOMY_FAMILIES])(
    'drops the /c/ segment on %s detail, keeping the id',
    (family) => {
      expect(resolveRenamedTaxonomyPath(`/galgame-${family}/c/1234`)).toBe(
        `/galgame/${family}/1234`
      )
    }
  )

  it('leaves live and unrelated paths alone', () => {
    // The current pages must never be rewritten — that would be a redirect loop.
    expect(resolveRenamedTaxonomyPath('/galgame/tag/55')).toBeNull()
    expect(resolveRenamedTaxonomyPath('/galgame/tag')).toBeNull()
    // Product-face routes keep their kebab namespace and are out of scope.
    expect(resolveRenamedTaxonomyPath('/galgame-resource')).toBeNull()
    expect(resolveRenamedTaxonomyPath('/galgame-rating/12')).toBeNull()
    // A gen-1 wiki-id URL is NOT this function's job (it needs a map lookup).
    expect(resolveRenamedTaxonomyPath('/galgame-tag/5')).toBeNull()
    // Malformed / deeper shapes fall through rather than guessing.
    expect(resolveRenamedTaxonomyPath('/galgame-tag/c/abc')).toBeNull()
    expect(resolveRenamedTaxonomyPath('/galgame-tag/c/5/extra')).toBeNull()
    expect(resolveRenamedTaxonomyPath('/galgame-series/c/5')).toBeNull()
  })
})

describe('gen-1 wiki-id → current, one hop (no 301 chain)', () => {
  it('resolves a mapped tag straight to the final form', () => {
    // 1 → 55 in the frozen map.
    expect(resolveLegacyTag(1)).toEqual({
      kind: 'redirect',
      to: '/galgame/tag/55'
    })
  })

  it('resolves a non-identity engine straight to the final form', () => {
    // 140 → 138: one of the 52 engines that did NOT keep its id, so this also
    // pins that the map is consulted rather than the id passed through.
    expect(resolveLegacyEngine(140)).toEqual({
      kind: 'redirect',
      to: '/galgame/engine/138'
    })
  })

  it('never emits an intermediate form for ANY mapped tag', () => {
    const bad = Object.keys(tagRedirects as Record<string, number>)
      .map((wikiId) => resolveLegacyTag(Number(wikiId)))
      .filter((r) => r.kind !== 'redirect' || !isFinalForm(r.to))
    expect(bad).toEqual([])
  })

  it('never emits an intermediate form for ANY mapped engine', () => {
    const bad = Object.keys(engineRedirects as Record<string, number>)
      .map((wikiId) => resolveLegacyEngine(Number(wikiId)))
      .filter((r) => r.kind !== 'redirect' || !isFinalForm(r.to))
    expect(bad).toEqual([])
  })

  it('410s a retired tag and 404s an unknown one', () => {
    const gone = (tagGone as number[])[0]!
    expect(resolveLegacyTag(gone)).toEqual({ kind: 'gone' })
    // Far past both id ranges: never a valid id in the first place.
    expect(resolveLegacyTag(99_999_999)).toEqual({ kind: 'missing' })
    expect(resolveLegacyEngine(99_999_999)).toEqual({ kind: 'missing' })
  })
})

describe('parseWikiId', () => {
  it('accepts positive integers only', () => {
    expect(parseWikiId('5')).toBe(5)
    expect(parseWikiId('0')).toBeNull()
    expect(parseWikiId('-3')).toBeNull()
    expect(parseWikiId('1.5')).toBeNull()
    expect(parseWikiId('abc')).toBeNull()
    expect(parseWikiId(undefined)).toBeNull()
  })
})

// A merged catalog id (doc 148) is the THIRD generation of stale taxonomy URL,
// and the only one resolved at runtime rather than from a frozen map: the
// catalog answers the detail request with `moved_to`, and the page hops there.
// It obeys the same one-hop rule as the retired shells — which is only true
// because the target comes from the shared builder rather than being spelled
// out at the call site.
describe('merged catalog id hop', () => {
  it.each([...TAXONOMY_FAMILIES])(
    'builds a final-form %s target from a survivor id',
    (family) => {
      const to = taxonomyDetailPath(family, 6935)
      expect(isFinalForm(to)).toBe(true)
      expect(to).toBe(`/galgame/${family}/6935`)
    }
  )

  it('lands on a URL that no other rule would redirect again', () => {
    const to = taxonomyDetailPath('official', 6935)
    // The retired-shell resolver only knows the `/galgame-…` space, so a
    // current-form URL falls straight through it — no second hop exists.
    expect(resolveRenamedTaxonomyPath(to)).toBeNull()
  })
})

// The 500 that shipped with the hop (prod: /galgame/official/13323).
//
// `navigateTo` in a page's setup is not an early return: on SSR it parks a
// redirect response and lets setup + the template finish, so a page that keeps
// rendering does it against the tombstone payload (alias / galgame null) and
// the resulting throw preempts the parked 301 with a 500. Nothing in the type
// system says "stop here", so the guard is a source-shape invariant: a
// taxonomy detail page that hops MUST also gate its template on the tombstone.
describe('merged-id pages stop rendering after the hop', () => {
  const pageSource = (family: TaxonomyFamily) =>
    // Vitest runs with apps/web as the cwd; `import.meta.url` is rewritten by
    // the Nuxt/Vite transform and does not point at this file on disk.
    readFileSync(
      resolve(process.cwd(), `app/pages/galgame/${family}/[id].vue`),
      'utf-8'
    )

  it.each([...TAXONOMY_FAMILIES])(
    '%s: a navigateTo in setup comes with a template gate',
    (family) => {
      const source = pageSource(family)
      // tag / engine are not merge-capable today, so they have no hop at all —
      // the day one grows a navigateTo, this assertion starts applying to it.
      if (!source.includes('navigateTo(')) {
        expect(source).not.toContain('moved_to')
        return
      }
      const root = source.match(/<template>\s*\n\s*<div ([^>]*)>/)
      expect(root?.[1]).toContain('!data.moved_to')
    }
  )

  it('official: the SEO block is inside the not-moved gate', () => {
    const source = pageSource('official')
    const gate = source.indexOf('if (!moved) {')
    expect(gate).toBeGreaterThan(-1)
    expect(source.indexOf('useKunSeoMeta(')).toBeGreaterThan(gate)
  })
})
