// The current viewer's favorited topic ids + the reaction keys they hold per
// topic. The activity feed is shared-cached (no per-viewer state in the payload),
// so feed cards hydrate "did I favorite / react" from here — one fetch per
// session, mirroring useMyGalgameInteractions.
export const useMyTopicInteractions = () => {
  const { id } = usePersistUserStore()
  const favorited = useState<number[]>('my-topic-favorited', () => [])
  const reactions = useState<Record<number, string[]>>(
    'my-topic-reactions',
    () => ({})
  )
  const loaded = useState<boolean>('my-topic-interactions-loaded', () => false)

  const favoritedSet = computed(() => new Set(favorited.value))

  const ensureLoaded = async () => {
    if (loaded.value || !id) return
    loaded.value = true
    const res = await kunFetch<{
      favorited: number[]
      reactions: Record<string, string[]>
    }>('/topic/interactions/mine')
    if (!res) return
    favorited.value = res.favorited ?? []
    // JSON object keys arrive as strings — normalise to number-keyed.
    const norm: Record<number, string[]> = {}
    for (const [k, v] of Object.entries(res.reactions ?? {})) norm[Number(k)] = v
    reactions.value = norm
  }

  return {
    isFavorited: (tid: number) => favoritedSet.value.has(tid),
    reactionKeysOf: (tid: number) => reactions.value[tid] ?? [],
    ensureLoaded
  }
}
