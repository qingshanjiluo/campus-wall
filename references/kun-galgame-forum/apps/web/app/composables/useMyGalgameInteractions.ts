// The current user's liked / favorited galgame ids, fetched once per session
// (client-side, logged-in only). The activity feed is cached SHARED across users
// (keyed by SFW setting, not per-user), so it can't carry the viewer's own
// like/favorite state — feed galgame cards hydrate their buttons from this
// instead. Shared via useState so every card reads one cache + one fetch.
export const useMyGalgameInteractions = () => {
  const { id } = usePersistUserStore()
  const liked = useState<number[]>('my-galgame-liked', () => [])
  const favorited = useState<number[]>('my-galgame-favorited', () => [])
  const loaded = useState<boolean>('my-galgame-interactions-loaded', () => false)

  const ensureLoaded = async () => {
    if (loaded.value || !id) return
    loaded.value = true // claim early so concurrent cards don't double-fetch
    const res = await kunFetch<{ liked: number[]; favorited: number[] }>(
      '/galgame/interactions/mine'
    )
    if (res) {
      liked.value = res.liked ?? []
      favorited.value = res.favorited ?? []
    } else {
      loaded.value = false // let a later mount retry
    }
  }

  const likedSet = computed(() => new Set(liked.value))
  const favoritedSet = computed(() => new Set(favorited.value))

  // Keep the shared favorited set in sync after the collection picker commits,
  // so every feed card (and a later navigation) reflects the change without a
  // refetch. "favorited" = the game is in >=1 of the user's collections.
  const setFavorited = (gid: number, isFav: boolean) => {
    const set = new Set(favorited.value)
    if (isFav) {
      set.add(gid)
    } else {
      set.delete(gid)
    }
    favorited.value = [...set]
  }

  return {
    isLiked: (gid: number) => likedSet.value.has(gid),
    isFavorited: (gid: number) => favoritedSet.value.has(gid),
    setFavorited,
    ensureLoaded
  }
}
