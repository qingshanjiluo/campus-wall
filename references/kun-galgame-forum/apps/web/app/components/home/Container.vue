<script setup lang="ts">
import { useIntersectionObserver, useThrottleFn } from '@vueuse/core'

// NOTE: the template must keep a SINGLE root element with NO sibling top-level
// comment — index.vue renders this with `keepalive: true`, and a leading
// template comment compiles to a multi-root Fragment, which warns under
// <KeepAlive>. (The grid rationale lives inside the root div, not above it.)

// Home page = the activity feed. Tabs are USER-CONFIGURABLE (设置 → 动态): each tab
// is a set of activity "kinds" stored in the settings store, sent to the backend
// as GET /activity/tab?types=…. The defaults replicate the previous fixed five
// (全部 / 话题 / Galgame / 资源和求助 / 其他). Cards reuse the /activity card style;
// no KunCard wrapper here (the feed is the page).
const settings = usePersistSettingsStore()
const { feedTabs } = storeToRefs(settings)
const tabItems = computed(() =>
  feedTabs.value.map((t) => ({ value: t.id, textValue: t.name, icon: t.icon }))
)
// Tab lives in the URL (?tab=) so it survives back/forward, refresh + sharing.
const activeTab = useTabQuery(feedTabs.value[0]?.id ?? 'all')
// The active tab's kinds → the `types` query param (comma-joined). Falls back to
// the first tab if the active id was deleted/renamed away in settings.
const activeTypes = computed(() => {
  const tab =
    feedTabs.value.find((t) => t.id === activeTab.value) ?? feedTabs.value[0]
  return (tab?.kinds ?? []).join(',')
})
// If the active tab vanished (deleted in settings), snap back to the first tab.
watchEffect(() => {
  if (
    feedTabs.value.length &&
    !feedTabs.value.some((t) => t.id === activeTab.value)
  ) {
    activeTab.value = feedTabs.value[0]!.id
  }
})

const items = ref<ActivityItem[]>([])
const cursor = ref('')
const hasMore = ref(true)
const isLoadingMore = ref(false)

// Cap consecutive AUTO-loads so a fast scroll can't pull page after page (each
// page is a heavy backend enrichment); past the cap the 加载更多 button takes over,
// and a manual click resumes auto-loading for another batch.
const MAX_AUTO_LOADS = 4
const autoLoadCount = ref(0)
// Aborts the in-flight page on a tab switch / unmount so the backend isn't left
// enriching a page nobody will see.
let controller: AbortController | null = null

// First page is SSR-rendered (SEO, no spinner). The reactive query re-fetches
// when the tab or the show_no_resource toggle changes; the watch re-seeds the
// accumulator from whichever first page is current.
const { data, status } = await useKunFetch<{
  items: ActivityItem[]
  next_cursor: string
}>('/activity/tab', {
  method: 'GET',
  query: computed(() => ({
    types: activeTypes.value,
    limit: 30,
    show_no_resource: settings.showKUNGalgameNoResource,
    // 全部 tab is always SFW — NSFW topics + galgame activity are filtered out of
    // the main stream regardless of the viewer's NSFW setting.
    force_sfw: activeTab.value === 'all'
  }))
})

watch(
  data,
  (page) => {
    if (!page) return
    items.value = page.items
    cursor.value = page.next_cursor
    hasMore.value = !!page.next_cursor
  },
  { immediate: true }
)

// On a tab switch, drop the stale cursor immediately so an in-flight infinite
// scroll can't append the OLD tab's next page onto the NEW tab (the data watch
// re-seeds once the new first page arrives; old items stay until then — no flash).
watch(activeTab, () => {
  cursor.value = ''
  hasMore.value = true
  autoLoadCount.value = 0
  controller?.abort()
})

const loadMore = async (auto = false) => {
  if (isLoadingMore.value || !hasMore.value || !cursor.value) return
  if (auto) {
    if (autoLoadCount.value >= MAX_AUTO_LOADS) return // cap → wait for a click
    autoLoadCount.value++
  } else {
    autoLoadCount.value = 0 // a manual 加载更多 resumes auto-loading
  }
  const tab = activeTab.value
  const types = activeTypes.value
  isLoadingMore.value = true
  controller = new AbortController()
  const next = await kunFetch<{ items: ActivityItem[]; next_cursor: string }>(
    '/activity/tab',
    {
      method: 'GET',
      query: {
        types,
        limit: 30,
        cursor: cursor.value,
        show_no_resource: settings.showKUNGalgameNoResource,
        force_sfw: tab === 'all'
      },
      signal: controller.signal
    }
  )
  isLoadingMore.value = false
  // Discard a stale page: aborted (kunFetch → null), or the tab changed mid-flight.
  if (!next || activeTab.value !== tab) return
  items.value.push(...next.items)
  cursor.value = next.next_cursor
  hasMore.value = !!next.next_cursor
}

// Auto-load near the bottom, THROTTLED so a fast scroll fires at most ~once/600ms
// instead of page-after-page (VueUse no-ops on SSR). The 加载更多 button is the
// manual fallback once the auto-load cap is hit.
const autoLoad = useThrottleFn(() => loadMore(true), 600)
onBeforeUnmount(() => controller?.abort())

const sentinel = ref<HTMLElement | null>(null)
useIntersectionObserver(
  sentinel,
  ([entry]) => {
    if (entry?.isIntersecting) autoLoad()
  },
  { rootMargin: '150px' }
)
</script>

<template>
  <div
    class="grid grid-cols-1 items-start gap-4 sm:grid-cols-[7rem_minmax(0,1fr)] lg:grid-cols-[7rem_minmax(0,1fr)_18rem] xl:grid-cols-[7rem_minmax(0,1fr)_20rem]"
  >
    <!-- Grid (not flex) so all column tracks exist the moment the container is
         parsed, BEFORE the children stream in. The right track is reserved empty
         up front, so on a slow connection the center feed can't grow into it and
         then reflow when the aside finally arrives — kills the layout shift. The
         track widths mirror the children (7rem=w-28 rail, 18/20rem=w-72/80 aside,
         1fr center). `display:none` children (mobile tabs / hidden rails) don't
         occupy a track, so the mobile→sm→lg responsive steps are unchanged. -->

    <!-- Mobile: horizontal underline tabs on top. `scrollable` (not full-width)
         so the user-configurable tab set scrolls sideways instead of overflowing
         the viewport once there are more tabs than fit. -->
    <div class="sm:hidden">
      <KunTab
        v-model="activeTab"
        :items="tabItems"
        variant="underlined"
        color="primary"
        scrollable
      />
    </div>

    <!-- Desktop: vertical underline tab rail on the left (like the settings
         panel). Sticky so it stays put while the center feed scrolls. -->
    <div class="sticky top-20 hidden self-start sm:block">
      <KunTab
        v-model="activeTab"
        :items="tabItems"
        orientation="vertical"
        variant="underlined"
        color="primary"
        full-width
      />
    </div>

    <!-- Dim the (stale) feed while a tab switch / filter refetch is in flight,
         so the content clearly lags-then-updates instead of silently sitting on
         the previous tab's items. The tab rails above stay lit. -->
    <KunLoadingDim
      class="min-w-0"
      :loading="status === 'pending' && items.length > 0"
    >
      <KunNull
        v-if="status !== 'pending' && !items.length"
        description="暂无动态"
      />

      <div v-else class="divide-default-200/60 divide-y">
        <div
          v-for="activity in items"
          :key="activity.unique_id"
          class="py-5 first:pt-0 last:pb-0"
        >
          <ActivityCard :activity="activity" />
        </div>
        <!-- Skeleton while a page loads — generic feed-card placeholders. -->
        <template v-if="isLoadingMore">
          <div v-for="n in 3" :key="`skeleton-${n}`" class="py-5">
            <ActivityCardSkeleton />
          </div>
        </template>
      </div>

      <div v-if="items.length" ref="sentinel" class="flex justify-center pt-4">
        <KunButton
          v-if="hasMore && !isLoadingMore"
          variant="light"
          @click="loadMore(false)"
        >
          加载更多
        </KunButton>
        <span v-else-if="!hasMore" class="text-default-400 text-sm">
          没有更多动态了
        </span>
      </div>
    </KunLoadingDim>

    <!-- Right rail (desktop only, ≥lg): carousel · 使用提示 at the top, footer
         pinned to the bottom. Sticky + viewport-tall so it stays fixed while the
         center feed scrolls; fixed width keeps the feed the focus (~65-70%). -->
    <aside
      class="sticky top-20 hidden h-[calc(100dvh-6rem)] flex-col self-start lg:flex"
    >
      <div class="space-y-4">
        <HomeCarousel />
        <HomeAsideHelp />
      </div>
      <HomeFooter class="mt-auto" />
    </aside>
  </div>
</template>
