import { useRouteQuery } from '@vueuse/router'
import type {
  KunGalgameResourceTypeOptions,
  KunGalgameResourceLanguageOptions,
  KunGalgameResourcePlatformOptions
} from '~/constants/galgame'

type SortField =
  | 'time'
  | 'created'
  | 'view'
  | 'view_1d'
  | 'view_7d'
  | 'view_30d'
  | 'release_date'
  | 'rating'

// Single source of truth for the galgame browse filters, backed by the
// URL query rather than a Pinia store. Every component that calls this
// (Nav, Container, the tag/official/engine detail pages) reads & writes
// the same query keys, so they stay in sync via the route — and the
// filtered view becomes shareable, bookmarkable, and survives refresh +
// browser back/forward for free.
//
// Why @vueuse/router's useRouteQuery (not a hand-rolled useRoute +
// router.replace): it batches writes within a tick, so a filter change
// that ALSO resets the page (the watcher in Nav) doesn't clobber itself
// via two racing router.replace calls reading stale route.query.
//
// `mode: 'replace'` keeps filter tweaks out of the history stack (a back
// press leaves the list, not steps through every chip toggle). Values
// equal to their default are dropped from the URL, so an unfiltered
// browse stays a clean `/galgame`.
//
// URL keys deliberately match the BE /galgame query keys, so Container
// can spread these straight into useKunFetch with no key remapping.
// Multi-value dimensions (months, providers) ride as CSV strings — the
// BE already parses them via ParseMonthSet / splitCSV, and a CSV string
// hits useRouteQuery's scalar default-omission cleanly (an empty array
// would not compare equal to its default by reference).
export const useGalgameFilters = () => {
  const opts = { mode: 'replace' as const }

  // Page is URL-backed too (shareable "page 3 of these filters"); default
  // 1 is omitted. Number transform so the ref is a number, not a string.
  const page = useRouteQuery('page', 1, { ...opts, transform: Number })

  const type = useRouteQuery<KunGalgameResourceTypeOptions>('type', 'all', opts)
  const language = useRouteQuery<KunGalgameResourceLanguageOptions>(
    'language',
    'all',
    opts
  )
  const platform = useRouteQuery<KunGalgameResourcePlatformOptions>(
    'platform',
    'all',
    opts
  )
  // 作品类型 — the rating-derived work type (ba_saku/plot/moe/daily), plus
  // 'uncategorized' (no rating tagged the game). 'all' = no filter (URL-omitted).
  const gameType = useRouteQuery<string>('gameType', 'all', opts)
  const sortField = useRouteQuery<SortField>('sortField', 'time', opts)
  const sortOrder = useRouteQuery<KunOrder>('sortOrder', 'desc', opts)

  // Release-date filter (wiki §17 / §17.10).
  //   releasedFrom / releasedTo — year bounds ('' | 'YYYY')
  //   releasedMonths           — discontinuous month set, CSV of 1-12
  // Explicit <string> generic: without it useRouteQuery infers the literal
  // type `""` from the default, so assigning any other string (a picked year /
  // CSV) fails to type-check.
  const releasedFrom = useRouteQuery<string>('releasedFrom', '', opts)
  const releasedTo = useRouteQuery<string>('releasedTo', '', opts)
  const releasedMonths = useRouteQuery<string>('releasedMonths', '', opts)

  // Provider filters as CSV strings (BE splitCSV-friendly). Toggling is
  // done on the CSV in the UI, same pattern as releasedMonths.
  const includeProviders = useRouteQuery<string>('includeProviders', '', opts)
  const excludeOnlyProviders = useRouteQuery<string>(
    'excludeOnlyProviders',
    '',
    opts
  )

  // Bayesian-rating advanced filters. minRatingCount = high-confidence
  // gate (>= N votes); minRating = Bayesian score >= X (0–10). 0 = off
  // (omitted from the URL). Sort by rating uses sortField='rating'.
  const minRatingCount = useRouteQuery('minRatingCount', 0, {
    ...opts,
    transform: Number
  })
  const minRating = useRouteQuery('minRating', 0, {
    ...opts,
    transform: Number
  })

  // Fixed page size — not user-facing, kept off the URL.
  const limit = 24

  return {
    page,
    limit,
    type,
    language,
    platform,
    gameType,
    sortField,
    sortOrder,
    releasedFrom,
    releasedTo,
    releasedMonths,
    includeProviders,
    excludeOnlyProviders,
    minRatingCount,
    minRating
  }
}
