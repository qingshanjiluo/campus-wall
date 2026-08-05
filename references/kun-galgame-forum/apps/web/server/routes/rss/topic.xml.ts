import { fetchKunApi, useKunFeed } from '../../utils/kunFeed'

interface TopicRSSItem {
  id: number
  title: string
  description: string
  user_id: number
  user_name: string
  created: string
}

// Cache the WHOLE RSS response (not just the upstream fetch). RSS aggregators
// poll feeds relentlessly (every few minutes, 24/7, from many clients), so
// /rss/*.xml are by far the highest-volume endpoints. Previously the HANDLER ran
// per poll (Feed build + rss2 + an SSR $fetch to kungal-api) — at that volume,
// per-poll handler work + the upstream $fetch were the memory/socket leak
// amplifier on kungal-web. Caching the handler collapses the flood into ONE real
// render per maxAge window; polls are served straight from cache.
//
// Failure semantics (replaces the old try/catch empty-feed): a slow/unreachable
// kungal-api makes fetchKunApi throw, so the failed render is NOT cached. With
// swr + a long staleMaxAge, the last-good feed keeps being served while the
// background refresh retries — graceful degradation, no empty-feed caching and
// no per-poll 500 flood. Only a truly cold cache + simultaneous outage 500s.
export default defineCachedEventHandler(
  async (event): Promise<string> => {
    const baseUrl = useRuntimeConfig().public.KUN_GALGAME_URL || ''
    const feed = useKunFeed(baseUrl, 'topic')

    const topics = await fetchKunApi<TopicRSSItem[]>('/rss/topic')
    for (const t of topics) {
      feed.addItem({
        link: `${baseUrl}/topic/${t.id}`,
        title: t.title,
        date: new Date(t.created),
        description: t.description,
        author: [
          {
            name: t.user_name,
            link: `${baseUrl}/user/${t.user_id}/info`
          }
        ]
      })
    }

    setHeader(event, 'Content-Type', 'application/xml')
    return feed.rss2()
  },
  {
    name: 'rss-topic',
    getKey: () => 'all',
    swr: true,
    maxAge: 60 * 5, // fresh window: one real render per 5 min
    staleMaxAge: 60 * 60 * 24 * 7 // serve last-good feed up to 7d during an upstream outage
  }
)
