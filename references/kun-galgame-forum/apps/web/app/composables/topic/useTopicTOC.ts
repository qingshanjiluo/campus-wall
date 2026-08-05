import { ref, computed } from 'vue'
import type { InjectionKey } from 'vue'

export interface TOCItem {
  id: string
  text: string
  level: number
  type: 'heading' | 'reply'
  // True for the reply the viewer deep-linked to (a ?comment resolves to its
  // parent reply); the rail renders it persistently bold so they can spot where
  // they landed even after scrolling away.
  targeted?: boolean
}

// Getters (not refs) so the provider can hand over plain reactive reads without
// running into Ref variance — the computed below tracks whatever they touch.
export interface TopicTocSource {
  getContentHtml: () => string
  getReplies: () => { floor: number; content_markdown: string }[]
  // The currently-active reply floor — the deep-link target, which also follows
  // same-page #floor / feed-card jumps (a ?comment resolves to its parent reply's
  // floor); a getter so the rail's bold reactively tracks it. 0 = none.
  getTargetFloor?: () => number
}

export const TOPIC_TOC_SOURCE: InjectionKey<TopicTocSource> =
  Symbol('topicTocSource')

// Pixels at the top of the viewport hidden behind the fixed top bar — a section
// whose whole band sits above this line counts as already scrolled past.
const TOP_BAR_OFFSET = 88

// SSR-safe text extraction: strip tags + decode the entities goldmark emits, so
// the rail text equals what `textContent` would yield on the client — keeping
// the server and client renders byte-identical (no hydration mismatch).
const htmlToText = (html: string) =>
  html
    .replace(/<[^>]+>/g, '')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .trim()

const HEADING_RE = /<h([1-3])\b[^>]*\bid="([^"]*)"[^>]*>([\s\S]*?)<\/h\1>/gi

export const useTopicTOC = (source: TopicTocSource) => {
  // The LIST is derived from data (the server-rendered content HTML + the SSR'd
  // replies), NOT the DOM, so it renders on the server and hydrates without a
  // flash. The scrollspy further down is the client-only enhancement — the
  // canonical "server-render then enhance" pattern for a TOC.
  // https://blog.maximeheckel.com/posts/scrollspy-demystified/
  const headings = computed<TOCItem[]>(() => {
    const items: TOCItem[] = []

    // goldmark renders <h1..3 id="slug">text</h1..3> (parser.WithAutoHeadingID),
    // so the ids here match the ids in the live DOM — anchors + scrollspy align.
    for (const m of source.getContentHtml().matchAll(HEADING_RE)) {
      items.push({
        id: m[2]!,
        level: Number(m[1]),
        text: htmlToText(m[3]!),
        type: 'heading'
      })
    }

    for (const reply of source.getReplies()) {
      // Mirror Reply.vue's anchor id (`${floor}.${slug}`) exactly.
      const slug = markdownToText(reply.content_markdown).slice(0, 20)
      items.push({
        id: `${reply.floor}.${slug}`,
        level: 2,
        text: slug ? `${reply.floor}. ${slug}` : `${reply.floor}`,
        type: 'reply',
        targeted: !!source.getTargetFloor && source.getTargetFloor() === reply.floor
      })
    }

    return items
  })

  // ── client-only scrollspy ─────────────────────────────────────────────────
  // Every item whose content band overlaps the viewport, in document order.
  const activeIds = ref<string[]>([])
  let headingEls: HTMLElement[] = []
  let replyEls: HTMLElement[] = []
  let masterEl: HTMLElement | null = null
  let ticking = false

  const computeActive = () => {
    ticking = false
    const top = TOP_BAR_OFFSET
    const bottom = window.innerHeight
    const ids: string[] = []

    // Each item owns the band [its top, the next item's top); the last item in a
    // group runs to the group's end. A section taller than the viewport stays
    // active after its heading scrolls off, because its band still straddles it.
    const scan = (els: HTMLElement[], groupEnd: () => number) => {
      for (let i = 0; i < els.length; i++) {
        const el = els[i]!
        const next = els[i + 1]
        const bandTop = el.getBoundingClientRect().top
        const bandBottom = next ? next.getBoundingClientRect().top : groupEnd()
        if (bandTop < bottom && bandBottom > top) {
          ids.push(el.id)
        }
      }
    }

    scan(headingEls, () =>
      masterEl ? masterEl.getBoundingClientRect().bottom : bottom
    )
    scan(replyEls, () =>
      replyEls.length
        ? replyEls[replyEls.length - 1]!.getBoundingClientRect().bottom
        : bottom
    )

    const changed =
      ids.length !== activeIds.value.length ||
      ids.some((id, i) => id !== activeIds.value[i])
    if (changed) {
      activeIds.value = ids
    }
  }

  const onScroll = () => {
    if (ticking) {
      return
    }
    ticking = true
    requestAnimationFrame(computeActive)
  }

  // Re-query the live elements (after replies load more, fonts/images reflow…)
  // and recompute. The element ids match the data-derived list above.
  const refreshTOC = () => {
    headingEls = Array.from(
      document.querySelectorAll<HTMLElement>(
        '.kun-master h1, .kun-master h2, .kun-master h3'
      )
    )
    replyEls = Array.from(document.querySelectorAll<HTMLElement>('.kun-reply'))
    masterEl = document.querySelector<HTMLElement>('.kun-master')
    computeActive()
  }

  onMounted(() => {
    refreshTOC()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll, { passive: true })
  })

  onBeforeUnmount(() => {
    window.removeEventListener('scroll', onScroll)
    window.removeEventListener('resize', onScroll)
  })

  // The list grows as replies load — re-query the DOM once the new nodes render.
  watch(headings, () => nextTick(refreshTOC), { flush: 'post' })

  return {
    headings,
    activeIds,
    refreshTOC
  }
}
