import type { Ref } from 'vue'

export const useTopicReplies = (topicId: number | Ref<number>) => {
  const _topicId = toValue(topicId)

  const replies = useState<TopicReply[]>(
    `kun-topic-replies-${_topicId}`,
    () => []
  )
  const isComplete = useState<boolean>(
    `kun-topic-replies-complete-${_topicId}`,
    () => false
  )

  const status = useState<'idle' | 'pending' | 'success' | 'error'>(
    `kun-topic-replies-status-${_topicId}`,
    () => 'idle'
  )

  // Contiguous loaded page window [minPage, maxPage]. useState (not a plain ref)
  // so a deep-link's SSR-loaded page survives hydration — a ref would reset to 1
  // on the client and break load-earlier / load-more.
  const minPage = useState<number>(`kun-topic-replies-min-${_topicId}`, () => 1)
  const maxPage = useState<number>(`kun-topic-replies-max-${_topicId}`, () => 1)
  const sortOrder = useState<'asc' | 'desc'>(
    `kun-topic-replies-sort-${_topicId}`,
    () => 'asc'
  )

  // True when the window doesn't reach page 1 — i.e. a deep-link landed deeper and
  // there are earlier replies to pull in above.
  const hasEarlier = computed(() => minPage.value > 1)

  const _fetchReplies = async (
    fetchPage: number,
    fetchSortOrder: 'asc' | 'desc'
  ) => {
    status.value = 'pending'

    const newReplies = await kunFetch<TopicReply[]>(
      `/topic/${_topicId}/reply`,
      {
        query: {
          topic_id: _topicId,
          page: fetchPage,
          limit: 30,
          sort_order: fetchSortOrder
        }
      }
    )
    status.value = 'success'
    return newReplies ?? []
  }

  // Seed the window at a single starting page (1 by default, or a deep-link's
  // located page so SSR renders the target's page directly). Idempotent: skips if
  // replies are already loaded (e.g. SSR → client hydration), preserving the
  // SSR-set window.
  const loadInitialReplies = async (startPage = 1) => {
    if (replies.value.length > 0) {
      return
    }

    const page = Math.max(1, startPage)
    sortOrder.value = 'asc'
    minPage.value = page
    maxPage.value = page

    const data = await _fetchReplies(page, sortOrder.value)
    isComplete.value = data.length < 30
    replies.value = data
  }

  // Extend the window DOWN (next page, append).
  const loadMore = async () => {
    if (status.value === 'pending' || isComplete.value) return

    const next = maxPage.value + 1
    const newReplies = await _fetchReplies(next, sortOrder.value)
    maxPage.value = next
    if (newReplies.length < 30) {
      isComplete.value = true
    }
    replies.value.push(...newReplies)
  }

  // Extend the window UP (previous page, prepend). Powers the deep-link
  // "加载更早的回复". Browser scroll-anchoring keeps the viewport stable on prepend.
  const loadEarlier = async () => {
    if (status.value === 'pending' || minPage.value <= 1) return

    const prev = minPage.value - 1
    const newReplies = await _fetchReplies(prev, sortOrder.value)
    minPage.value = prev
    replies.value.unshift(...newReplies)
  }

  const setSort = async (order: 'asc' | 'desc') => {
    if (status.value === 'pending' || sortOrder.value === order) return

    sortOrder.value = order
    minPage.value = 1
    maxPage.value = 1
    isComplete.value = false

    const sortedReplies = await _fetchReplies(1, sortOrder.value)
    if (sortedReplies.length < 30) {
      isComplete.value = true
    }
    replies.value = sortedReplies
  }

  const addNewReply = (newReply: TopicReply) => {
    if (replies.value.some((r) => r.id === newReply.id)) return

    if (sortOrder.value === 'desc' && minPage.value === 1) {
      replies.value.unshift(newReply)
    } else {
      replies.value.push(newReply)
    }
  }

  const updateReply = (updatedReply: TopicReply) => {
    const index = replies.value.findIndex((r) => r.id === updatedReply.id)
    if (index !== -1) {
      replies.value[index] = updatedReply
    }
  }

  const removeReply = (replyId: number) => {
    const index = replies.value.findIndex((r) => r.id === replyId)
    if (index !== -1) {
      replies.value.splice(index, 1)
    }
  }

  return {
    replies,
    status,
    isComplete,
    hasEarlier,
    sortOrder,
    loadInitialReplies,
    loadMore,
    loadEarlier,
    setSort,
    addNewReply,
    updateReply,
    removeReply
  }
}
