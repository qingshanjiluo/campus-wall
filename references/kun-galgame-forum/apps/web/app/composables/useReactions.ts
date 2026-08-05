import type { InjectionKey, Ref, ComputedRef } from 'vue'

// Shared reaction state for a single topic / reply. The owning component
// (Master.vue / Reply.vue) creates it and provides it via reactionsKey so the
// chips (TopicReactionBar) and the trigger button (TopicReactionTrigger) — which
// may live apart, e.g. the trigger in the desktop footer — stay in sync.
export interface ReactionsState {
  list: Ref<KunReaction[]>
  mineKeys: ComputedRef<string[]>
  toggle: (key: string) => Promise<void>
  // Set only for a TOPIC's reactions (not replies) — gates the 查看历史 row + tells
  // the history modal which topic to fetch.
  topicId?: number
}

export const reactionsKey: InjectionKey<ReactionsState> = Symbol('reactions')

interface UseReactionsOptions {
  topicId?: number
  replyId?: number
  targetUserId: number
  reactions: KunReaction[]
  // Optional reactive source: re-seed the list when it emits (used by the feed
  // card, whose per-viewer "mine" hydrates after mount). Stops after the first
  // user toggle so it never clobbers an optimistic change.
  sync?: () => KunReaction[]
  // When true (topic/reply detail), chips show up to MAX_AVATARS reactor avatars
  // + a "+N" overflow, so every chip has the same height. The feed leaves this
  // off and renders emoji + count instead (it doesn't ship reactors).
  showReactors?: boolean
}

// Max reactor avatars shown per chip; the rest collapse to a "+N" badge.
const MAX_AVATARS = 3

export const useReactions = (opts: UseReactionsOptions): ReactionsState => {
  const { id, name, avatar } = usePersistUserStore()

  const clone = (rs: KunReaction[]): KunReaction[] =>
    rs.map((r) => ({ ...r, reactors: r.reactors ? [...r.reactors] : undefined }))

  const list = ref<KunReaction[]>(opts.reactions ? clone(opts.reactions) : [])
  const inflight = new Set<string>()

  // Re-seed from the reactive source (late "mine" hydration) until the viewer
  // first interacts — after that the local list is authoritative.
  let userTouched = false
  if (opts.sync) {
    watch(opts.sync, (v) => {
      if (!userTouched) list.value = clone(v)
    })
  }

  const mineKeys = computed(() =>
    list.value.filter((r) => r.mine).map((r) => r.reaction)
  )

  const post = (reaction: string) =>
    opts.topicId
      ? kunFetch(`/topic/${opts.topicId}/reaction`, {
          method: 'PUT',
          body: { reaction }
        })
      : kunFetch(`/topic/0/reply/reaction`, {
          method: 'PUT',
          body: { reply_id: opts.replyId, reaction }
        })

  const removeMine = (idx: number) => {
    const r = list.value[idx]!
    r.count--
    r.mine = false
    if (r.reactors) r.reactors = r.reactors.filter((u) => u.id !== id)
    if (r.count <= 0) list.value.splice(idx, 1)
  }

  const applyOptimistic = (key: string) => {
    const idx = list.value.findIndex((r) => r.reaction === key)
    if (idx >= 0 && list.value[idx]!.mine) {
      removeMine(idx)
    } else if (idx >= 0) {
      const r = list.value[idx]!
      r.count++
      r.mine = true
      if (opts.showReactors && (r.reactors?.length ?? 0) < MAX_AVATARS) {
        ;(r.reactors ??= []).push({ id, name, avatar })
      }
    } else {
      list.value.push({
        reaction: key,
        count: 1,
        mine: true,
        reactors: opts.showReactors ? [{ id, name, avatar }] : undefined
      })
    }
    // like ⇄ dislike are mutually exclusive — drop the opposite if it was mine.
    const opposite =
      key === 'like' ? 'dislike' : key === 'dislike' ? 'like' : null
    if (opposite) {
      const j = list.value.findIndex((r) => r.reaction === opposite)
      if (j >= 0 && list.value[j]!.mine) removeMine(j)
    }
  }

  const toggle = async (key: string) => {
    if (!id) {
      useAuthModal().open()
      return
    }
    if (key === 'like' && id === opts.targetUserId) {
      useMessage(10236, 'warn')
      return
    }
    if (inflight.has(key)) return
    inflight.add(key)
    userTouched = true

    const snapshot = clone(list.value)
    applyOptimistic(key)

    const ok = await post(key)
    if (!ok) list.value = snapshot
    inflight.delete(key)
  }

  return { list, mineKeys, toggle, topicId: opts.topicId }
}
