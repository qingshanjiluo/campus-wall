import { useRouteQuery } from '@vueuse/router'

// URL-backed filter state for the toolset list — the same pattern as
// useGalgameFilters (replaces the old useTempToolsetStore Pinia store).
// Every consumer (Nav, Container) reads/writes the same query keys, so
// the filtered toolset view is shareable, survives refresh, and supports
// browser back/forward. mode:'replace' keeps filter churn out of history;
// default values are dropped from the URL. URL keys === BE /toolset query
// keys, so Container spreads them straight into the fetch.
type ToolsetType =
  | 'all'
  | 'emulator'
  | 'translator'
  | 'extractor'
  | 'converter'
  | 'debug'
  | 'launcher'
  | 'script'
  | 'docs'
  | 'others'
type ToolsetLanguage = 'all' | 'ja-jp' | 'en-us' | 'zh-cn' | 'zh-tw' | 'others'
type ToolsetPlatform = 'all' | 'windows' | 'mac' | 'linux' | 'emulator' | 'others'
type ToolsetVersion = 'all' | 'alpha' | 'beta' | 'rc' | 'stable'
type ToolsetSortField = 'resource_update_time' | 'created' | 'view'

export const useToolsetFilters = () => {
  const opts = { mode: 'replace' as const }

  const page = useRouteQuery('page', 1, { ...opts, transform: Number })
  const type = useRouteQuery<ToolsetType>('type', 'all', opts)
  const language = useRouteQuery<ToolsetLanguage>('language', 'all', opts)
  const platform = useRouteQuery<ToolsetPlatform>('platform', 'all', opts)
  const version = useRouteQuery<ToolsetVersion>('version', 'all', opts)
  const sortField = useRouteQuery<ToolsetSortField>(
    'sortField',
    'resource_update_time',
    opts
  )
  const sortOrder = useRouteQuery<KunOrder>('sortOrder', 'desc', opts)

  const limit = 24

  return {
    page,
    limit,
    type,
    language,
    platform,
    version,
    sortField,
    sortOrder
  }
}
