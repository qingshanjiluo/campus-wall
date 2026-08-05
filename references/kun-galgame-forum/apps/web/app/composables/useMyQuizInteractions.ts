// The current user's favorited quiz ids, fetched once per session (client-side,
// logged-in only). The activity feed is cached SHARED across users (keyed by SFW
// setting, not per-user), so it can't carry the viewer's own favorite state —
// the feed 出题 card hydrates its 收藏 button from this. Mirrors
// useMyGalgameInteractions; shared via useState so every card reads one fetch.
export const useMyQuizInteractions = () => {
  const { id } = usePersistUserStore()
  const favorited = useState<number[]>('my-quiz-favorited', () => [])
  const loaded = useState<boolean>('my-quiz-interactions-loaded', () => false)

  const ensureLoaded = async () => {
    if (loaded.value || !id) return
    loaded.value = true // claim early so concurrent cards don't double-fetch
    const res = await kunFetch<{ favorited: number[] }>(
      '/galgame-quiz/mine/favorites'
    )
    if (res) {
      favorited.value = res.favorited ?? []
    } else {
      loaded.value = false // let a later mount retry
    }
  }

  const favoritedSet = computed(() => new Set(favorited.value))

  // Keep the shared set in sync after a card toggles, so every card (and a later
  // navigation) reflects the change without a refetch.
  const setFavorited = (quizId: number, isFav: boolean) => {
    const set = new Set(favorited.value)
    if (isFav) {
      set.add(quizId)
    } else {
      set.delete(quizId)
    }
    favorited.value = [...set]
  }

  return {
    isFavorited: (quizId: number) => favoritedSet.value.has(quizId),
    setFavorited,
    ensureLoaded
  }
}
