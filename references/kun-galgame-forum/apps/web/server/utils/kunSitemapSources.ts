// Builds the dynamic content URLs for the sitemap by enumerating the Go API's
// list endpoints. Consumed by server/api/__sitemap__/urls.ts, which @nuxtjs/sitemap
// pulls in as a `sources` entry (runtime generation, not build-time).
//
// SFW safety: we force the `KUNGalgameSettings` cookie to `sfw` on every API call
// so the BE applies its SFW filter (utils.IsSFW). NSFW detail URLs therefore never
// enter the sitemap. The BE already DEFAULTS to SFW when no cookie is present, but
// we send it explicitly so a future BE default change can't silently leak NSFW.
//
// Scale: the two big lists (resource ~24k, official ~23k) are capped at
// MAX_PAGES; all page fetches share one global concurrency limiter so a cold
// render can't flood the Go API. The whole thing is cached one layer up.
//
// Taxonomy ids: the list endpoints now return CATALOG ids (A2-3 / doc 106 R1),
// so the `/c/` locs below follow the new id space automatically — no map, no
// second source of truth. The legacy wiki-id URLs are never emitted here; they
// survive only as 301/410 shells for links already out in the world.

interface SitemapUrl {
  loc: string
  lastmod?: string
  changefreq?: string
  priority?: number
}

// The BE applies SFW item-by-item AFTER pagination, so SFW pages come back
// under-filled (e.g. 21/50) and `total` stays the UNFILTERED count. That's fine:
// for total-bearing lists we derive page count from the unfiltered total; for the
// topic list (no total) we walk until an empty page.
const SFW_COOKIE = `KUNGalgameSettings=${encodeURIComponent(
  JSON.stringify({ showKUNGalgameContentLimit: 'sfw' })
)}`

const PAGE_SIZE = 50 // BE hard cap — galgame/rating DTOs use validate:"max=50"
const MAX_PAGES = 130 // per-source ceiling (~6.5k rows); bounds a cold render
const GLOBAL_CONCURRENCY = 12 // API requests in flight across ALL sources at once

// A bounded queue shared by every apiGet so source-level parallelism can't
// multiply into a request storm against the Go API.
const createLimiter = (max: number) => {
  let active = 0
  const waiters: Array<() => void> = []
  const release = () => {
    active--
    waiters.shift()?.()
  }
  return async <T>(fn: () => Promise<T>): Promise<T> => {
    if (active >= max) await new Promise<void>((resolve) => waiters.push(resolve))
    active++
    try {
      return await fn()
    } finally {
      release()
    }
  }
}

const range = (from: number, to: number): number[] =>
  Array.from({ length: Math.max(0, to - from + 1) }, (_, i) => from + i)

const unwrap = (json: unknown): unknown =>
  (json as { data?: unknown })?.data ?? json

const toIso = (d: unknown): string | undefined => {
  if (typeof d === 'string' || typeof d === 'number' || d instanceof Date) {
    const t = new Date(d)
    if (!Number.isNaN(t.getTime())) return t.toISOString()
  }
  return undefined
}

// One paginated list endpoint → sitemap URLs.
//   pick:    pluck the row array from unwrapped data
//   total:   pluck the (unfiltered) total — omit for the topic list (walk mode)
//   loc:     row → frontend URL
//   lastmod: row → ISO date (optional; engine/official/tag have none)
interface PagedSource {
  path: string
  pick: (data: unknown) => Record<string, unknown>[]
  total?: (data: unknown) => number | undefined
  loc: (row: Record<string, unknown>) => string
  lastmod?: (row: Record<string, unknown>) => string | undefined
  priority: number
}

export const buildSitemapUrls = async (
  apiBase: string
): Promise<SitemapUrl[]> => {
  const limit = createLimiter(GLOBAL_CONCURRENCY)

  const apiGet = (path: string): Promise<unknown | null> =>
    limit(async () => {
      try {
        return await $fetch(`${apiBase}/api${path}`, {
          headers: { cookie: SFW_COOKIE, accept: 'application/json' },
          timeout: 15000
        })
      } catch {
        return null // a flaky endpoint drops its rows, never the whole sitemap
      }
    })

  const fetchPage = async (src: PagedSource, page: number) => {
    const body = await apiGet(`${src.path}?page=${page}&limit=${PAGE_SIZE}`)
    return body ? src.pick(unwrap(body)) : []
  }

  const toUrls = (src: PagedSource, rows: Record<string, unknown>[]): SitemapUrl[] =>
    rows.map((row) => ({
      loc: src.loc(row),
      lastmod: src.lastmod?.(row),
      changefreq: 'daily',
      priority: src.priority
    }))

  const collect = async (src: PagedSource): Promise<SitemapUrl[]> => {
    const firstBody = await apiGet(`${src.path}?page=1&limit=${PAGE_SIZE}`)
    if (!firstBody) return []
    const firstRows = src.pick(unwrap(firstBody))
    const urls = toUrls(src, firstRows)

    if (src.total) {
      const total = src.total(unwrap(firstBody))
      const pages =
        typeof total === 'number' && total > 0
          ? Math.min(Math.ceil(total / PAGE_SIZE), MAX_PAGES)
          : MAX_PAGES
      const rest = await Promise.all(
        range(2, pages).map((p) => fetchPage(src, p).then((r) => toUrls(src, r)))
      )
      for (const r of rest) urls.push(...r)
      return urls
    }

    // No total (topic list): walk in batches, stop at the first EMPTY page.
    // Under-filled SFW pages (length 1..49) are NOT the end, so we key on === 0.
    for (let start = 2; start <= MAX_PAGES; start += GLOBAL_CONCURRENCY) {
      const batch = await Promise.all(
        range(start, Math.min(start + GLOBAL_CONCURRENCY - 1, MAX_PAGES)).map((p) =>
          fetchPage(src, p)
        )
      )
      let reachedEnd = false
      for (const rows of batch) {
        urls.push(...toUrls(src, rows))
        if (rows.length === 0) reachedEnd = true
      }
      if (reachedEnd) break
    }
    return urls
  }

  // Single-GET lists (no pagination): engine (a few hundred hand-curated rows
  // the BE ships in one response).
  const collectSingle = async (
    path: string,
    pick: (data: unknown) => Record<string, unknown>[],
    loc: (row: Record<string, unknown>) => string,
    priority: number
  ): Promise<SitemapUrl[]> => {
    const body = await apiGet(path)
    if (!body) return []
    return pick(unwrap(body)).map((row) => ({
      loc: loc(row),
      changefreq: 'daily',
      priority
    }))
  }

  const num = (row: Record<string, unknown>, key: string) => row[key] as number

  const paged: PagedSource[] = [
    {
      path: '/topic',
      pick: (d) => (Array.isArray(d) ? (d as Record<string, unknown>[]) : []),
      loc: (r) => `/topic/${num(r, 'id')}`,
      // Typed cast (not a bare `r.foo`) so a future rename of this field is a
      // compile error here — these Record<string,unknown> rows are otherwise
      // outside vue-tsc's reach (the sitemap silently lost lastmod once already).
      lastmod: (r) => toIso((r as Pick<TopicCard, 'status_update_time'>).status_update_time),
      priority: 0.8
    },
    {
      path: '/galgame',
      pick: (d) => ((d as { galgames?: [] })?.galgames ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/galgame/${num(r, 'id')}`,
      lastmod: (r) => toIso((r as Pick<GalgameCard, 'resource_update_time'>).resource_update_time),
      priority: 0.8
    },
    {
      path: '/galgame-resource',
      pick: (d) => ((d as { resources?: [] })?.resources ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/galgame-resource/${num(r, 'id')}`,
      lastmod: (r) => toIso(r.edited ?? r.created),
      priority: 0.6
    },
    {
      path: '/galgame-rating/all',
      pick: (d) => ((d as { rating_data?: [] })?.rating_data ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/galgame-rating/${num(r, 'id')}`,
      lastmod: (r) => toIso(r.updated ?? r.created),
      priority: 0.6
    },
    {
      path: '/galgame-official',
      pick: (d) => ((d as { officials?: [] })?.officials ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/galgame/official/${num(r, 'id')}`,
      priority: 0.5
    },
    {
      // The tag list is server-PAGED (page/limit + total) and was collected
      // with one unpaginated GET, which took the BE default of 100 rows: the
      // sitemap has been offering ~100 of the ~1,700 tag pages, and the rest
      // were reachable only by internal links. It joins the paged sources so
      // the whole vocabulary is enumerated. This one list is the exception to
      // the note above: it pages out of a precomputed index, so its rows come
      // back full and its `total` is already SFW- and junk-filtered — the pages
      // enumerated here are exactly the tag pages that exist for a SFW visitor,
      // with no empty tail to walk.
      path: '/galgame-tag',
      pick: (d) => ((d as { tags?: [] })?.tags ?? []) as Record<string, unknown>[],
      total: (d) => (d as { total?: number })?.total,
      loc: (r) => `/galgame/tag/${num(r, 'id')}`,
      priority: 0.5
    }
  ]

  const groups = await Promise.all([
    ...paged.map((src) => collect(src)),
    collectSingle(
      '/galgame-engine',
      (d) => (Array.isArray(d) ? (d as Record<string, unknown>[]) : []),
      (r) => `/galgame/engine/${num(r, 'id')}`,
      0.5
    )
  ])

  // De-dup defensively (an id shouldn't repeat, but never emit a dup <url>).
  const seen = new Set<string>()
  const out: SitemapUrl[] = []
  for (const url of groups.flat()) {
    if (seen.has(url.loc)) continue
    seen.add(url.loc)
    out.push(url)
  }
  return out
}
