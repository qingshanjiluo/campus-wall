import type { Ref } from 'vue'

// Hydrates rendered quote chips (`<span class="kun-quote" data-reply-id data-floor>`)
// inside a content container into interactive references:
//   - click  → smooth-scroll to that floor's reply in-page + a brief highlight.
//   - hover  → lazily fetch the referenced reply and show a preview card.
// Mirrors useSpoilerContent's delegated-listener + flush:'post' watch pattern, so
// it re-binds whenever KunContent (re)renders. Cross-page jump is out of scope
// (replies are lazily paginated — see docs/proj/mention.md §10); a floor not on
// the current page surfaces a hint instead.

// Reply previews are immutable enough for a session — cache by id so re-hovering
// the same quote never refetches.
const previewCache = new Map<number, TopicReply | null>()

export interface QuotePreviewState {
  visible: boolean
  top: number
  left: number
  loading: boolean
  reply: TopicReply | null
}

export const useQuoteContent = (containerRef: Ref<HTMLElement | null>) => {
  const route = useRoute()
  const topicId = computed(
    () => Number((route.params as { id?: string }).id) || 0
  )

  const preview = reactive<QuotePreviewState>({
    visible: false,
    top: 0,
    left: 0,
    loading: false,
    reply: null
  })

  let hideTimer: ReturnType<typeof setTimeout> | null = null
  // Guards a stale fetch from overwriting a newer hover's result.
  let fetchSeq = 0

  const clearHideTimer = () => {
    if (hideTimer) {
      clearTimeout(hideTimer)
      hideTimer = null
    }
  }

  // Reply wrappers are anchored as `id="<floor>.<preview>"` (see Reply.vue), so
  // match on the `<floor>.` prefix — exact per floor (10. ≠ 1.). Highlight with
  // the same outline classes as the existing scrollPage helper for consistency.
  const FLASH = [
    'outline-2',
    'outline-offset-2',
    'outline-primary',
    'rounded-lg'
  ]
  const scrollToFloor = (floor: number) => {
    const el = document.querySelector<HTMLElement>(`[id^="${floor}."]`)
    if (!el) {
      useMessage('该楼层可能在其他分页，暂时无法跳转', 'info')
      return
    }
    el.scrollIntoView({ behavior: 'smooth', block: 'center' })
    el.classList.add(...FLASH)
    setTimeout(() => el.classList.remove(...FLASH), 1500)
  }

  const showPreview = async (el: HTMLElement, replyId: number) => {
    clearHideTimer()
    const rect = el.getBoundingClientRect()
    preview.top = rect.bottom + 8
    preview.left = rect.left
    preview.visible = true

    if (previewCache.has(replyId)) {
      preview.reply = previewCache.get(replyId) ?? null
      preview.loading = false
      return
    }

    preview.reply = null
    preview.loading = true
    const seq = ++fetchSeq
    const data = await kunFetch<TopicReply>(
      `/topic/${topicId.value}/reply/detail`,
      { method: 'GET', query: { replyId } }
    )
    if (seq !== fetchSeq) {
      return
    }
    previewCache.set(replyId, data)
    preview.reply = data
    preview.loading = false
  }

  // Debounced so moving the cursor from the chip onto the card (which cancels
  // the timer via keepPreview) doesn't flicker it away.
  const hidePreview = () => {
    clearHideTimer()
    hideTimer = setTimeout(() => {
      preview.visible = false
    }, 200)
  }

  const keepPreview = () => {
    clearHideTimer()
  }

  const quoteFrom = (e: Event) =>
    (e.target as HTMLElement | null)?.closest<HTMLElement>('.kun-quote') ?? null

  const onClick = (e: MouseEvent) => {
    const quote = quoteFrom(e)
    if (!quote) {
      return
    }
    e.preventDefault()
    const floor = Number(quote.dataset.floor)
    if (floor > 0) {
      scrollToFloor(floor)
    }
  }

  const onOver = (e: MouseEvent) => {
    const quote = quoteFrom(e)
    if (!quote) {
      return
    }
    const replyId = Number(quote.dataset.replyId)
    if (replyId > 0) {
      showPreview(quote, replyId)
    }
  }

  const onOut = (e: MouseEvent) => {
    if (quoteFrom(e)) {
      hidePreview()
    }
  }

  const setup = () => {
    const c = containerRef.value
    if (!c) {
      return
    }
    c.addEventListener('click', onClick)
    c.addEventListener('mouseover', onOver)
    c.addEventListener('mouseout', onOut)
  }

  const cleanup = () => {
    clearHideTimer()
    const c = containerRef.value
    if (!c) {
      return
    }
    c.removeEventListener('click', onClick)
    c.removeEventListener('mouseover', onOver)
    c.removeEventListener('mouseout', onOut)
  }

  watch(
    containerRef,
    (newEl, oldEl) => {
      if (oldEl) {
        cleanup()
      }
      if (newEl) {
        nextTick(setup)
      }
    },
    { flush: 'post' }
  )

  return { preview, keepPreview, hidePreview }
}
