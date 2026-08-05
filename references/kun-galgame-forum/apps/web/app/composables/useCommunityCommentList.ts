// Paging + grouping + optimistic mutation for a community-primitive comment area.
//
// Every comment area repeats the same list machinery: fetch page 1 through
// useKunFetch (so SSR and hydration work), seed local state once, grow it with a
// keyset cursor, group the flat arrival-ordered posts into two tiers, and splice
// optimistic create / edit / tombstone results in by id. That logic lived copied
// across the containers, which is how they drifted apart in the first place — it
// lives here now, so a new area is a descriptor entry plus a template.
//
// What stays in each container is only what genuinely differs: its page framing
// (card or bare, header copy, empty-state copy) and any area-specific extra, like
// the galgame area's legacy deep-link resolve.

export interface CommunityCommentGroup {
  root: GalgameCommunityComment
  replies: GalgameCommunityComment[]
}

// PAGE_LIMIT is the read page size. The server clamps to 50; 30 keeps the first
// screen cheap while still filling a typical thread in one request.
const PAGE_LIMIT = 30

// ASYNC ON PURPOSE — callers MUST `await` this. The await makes the calling
// component's setup async, which is what keeps SSR and hydration agreeing: the
// server then waits for the first page to settle before rendering, so it picks the
// same branch of the loading / empty / list chain that the client hydrates. Drop
// the await and the server renders KunLoading while the client hydrates KunNull —
// a "Hydration node mismatch" at KunNull (the galgame comment tab regressed exactly
// this way; every container in this family awaited before the logic moved here).
export const useCommunityCommentList = async (
  target: CommunityCommentTarget
) => {
  const surface = communityCommentSurface(target)

  const posts = ref<GalgameCommunityComment[]>([])
  const total = ref(0)
  const nextCursor = ref('')
  const seeded = ref(false)
  const loadingMore = ref(false)
  // locked mirrors the server's spoiler ruling for this viewer (only ever true on
  // a concealing quiz they have not answered). The rule itself is NOT re-derived
  // here — the server owns it, since the list is a public GET.
  const locked = ref(false)

  const { data, status } = await useKunFetch<GalgameCommunityCommentPage>(
    surface.listUrl,
    {
      lazy: true,
      method: 'GET',
      query: { ...surface.addressQuery, limit: PAGE_LIMIT }
    }
  )

  const seedFrom = (page: GalgameCommunityCommentPage) => {
    posts.value = [...page.posts]
    total.value = page.total
    nextCursor.value = page.next_cursor
    locked.value = page.locked
    seeded.value = true
  }

  // Seed once page 1 lands. Check the current value first (an SSR/hydrated payload
  // is already present during setup), then arm a NON-immediate watch for the lazy
  // case — never a self-stopping immediate watch, which is a TDZ crash (the
  // step-04 lesson, commit 47d2366c).
  if (data.value && !seeded.value) {
    seedFrom(data.value)
  }
  watch(data, (page) => {
    if (page && !seeded.value) {
      seedFrom(page)
    }
  })

  const hasMore = computed(() => nextCursor.value !== '')

  const loadMore = async () => {
    if (!hasMore.value || loadingMore.value) {
      return
    }
    loadingMore.value = true
    const page = await kunFetch<GalgameCommunityCommentPage>(surface.listUrl, {
      method: 'GET',
      query: {
        ...surface.addressQuery,
        cursor: nextCursor.value,
        limit: PAGE_LIMIT
      }
    })
    loadingMore.value = false
    if (page) {
      // De-dup against optimistic inserts already present in the list.
      const seen = new Set(posts.value.map((p) => p.id))
      posts.value = [
        ...posts.value,
        ...page.posts.filter((p) => !seen.has(p.id))
      ]
      nextCursor.value = page.next_cursor
      total.value = page.total
    }
  }

  // Two-tier grouping: root + one flat reply group. post_number is monotonic and a
  // root always precedes its replies in ascending keyset order, so forward paging
  // never orphans a reply; defensively, an orphan (deep-cursor edge) renders as its
  // own standalone group at its arrival position. A flat area (rating) never
  // nests, so every post is its own group.
  const groups = computed<CommunityCommentGroup[]>(() => {
    const list: CommunityCommentGroup[] = []
    const byRootId = new Map<number, CommunityCommentGroup>()
    for (const p of posts.value) {
      if (surface.isFlat || p.root_comment_id == null) {
        const group: CommunityCommentGroup = { root: p, replies: [] }
        byRootId.set(p.id, group)
        list.push(group)
      } else {
        const owner = byRootId.get(p.root_comment_id)
        if (owner) {
          owner.replies.push(p)
        } else {
          list.push({ root: p, replies: [] })
        }
      }
    }
    return list
  })

  const isEmpty = computed(
    () => seeded.value && !locked.value && total.value === 0
  )

  const handleNewComment = (post: GalgameCommunityComment) => {
    if (posts.value.some((p) => p.id === post.id)) {
      return
    }
    // Append in arrival order: a new root lands at the end; a new reply groups
    // under its (already-present) root. total tracks the thread's posts_count.
    posts.value = [...posts.value, post]
    total.value += 1
  }

  const handleUpdated = (updated: GalgameCommunityComment) => {
    // The edit response is built without the target_user enrichment (the server's
    // UpdateComment returns buildCommunityItem directly). An edit changes only the
    // body, never the reply relationship, so keep the prior "A → B" target.
    posts.value = posts.value.map((p) =>
      p.id === updated.id
        ? { ...updated, target_user: updated.target_user ?? p.target_user }
        : p
    )
  }

  const handleTombstoned = (postId: number) => {
    // Tombstone keeps the floor: flip the row to deleted and blank the body. The
    // thread's posts_count is unchanged (charter ruling 11), so total stays.
    posts.value = posts.value.map((p) =>
      p.id === postId
        ? { ...p, deleted: true, held: false, content: '', content_html: '' }
        : p
    )
  }

  // scrollToPost highlights a row by the area's anchor id — used for a
  // post-publish scroll and for deep-link resolution.
  const scrollToPost = (postId: number) => {
    nextTick(() => {
      setTimeout(() => {
        const el = document.getElementById(`${surface.anchorPrefix}-${postId}`)
        if (!el) {
          return
        }
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        const ring = [
          'outline-2',
          'outline-offset-2',
          'outline-primary',
          'rounded-lg'
        ]
        el.classList.add(...ring)
        setTimeout(() => el.classList.remove(...ring), 3000)
      }, 200)
    })
  }

  return {
    surface,
    posts,
    total,
    status,
    seeded,
    locked,
    hasMore,
    loadingMore,
    groups,
    isEmpty,
    loadMore,
    handleNewComment,
    handleUpdated,
    handleTombstoned,
    scrollToPost
  }
}
