<script setup lang="ts">
import { useIntersectionObserver } from '@vueuse/core'
import { KUN_ACTIVITY_TYPE_TYPE } from '~/constants/activity'

// Keyset (cursor) pagination + infinite scroll. The old numbered pages re-ran
// `ORDER BY created` (no tiebreaker) per page, so rows sharing a timestamp
// reordered between pages → duplicated / skipped entries. The API now returns a
// deterministic page plus an opaque `next_cursor`; we accumulate and seek.
const settings = usePersistSettingsStore()

const items = ref<ActivityItem[]>([])
const cursor = ref('')
const hasMore = ref(true)
const isLoadingMore = ref(false)

// First page renders on the server (SEO + no load spinner). The computed query
// re-fetches when the show_no_resource toggle flips; the watch below re-seeds the
// accumulator from whatever first page is current.
const { data, status } = await useKunFetch<{
  items: ActivityItem[]
  next_cursor: string
}>('/activity/timeline', {
  method: 'GET',
  query: computed(() => ({
    limit: 50,
    show_no_resource: settings.showKUNGalgameNoResource
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

const loadMore = async () => {
  if (isLoadingMore.value || !hasMore.value || !cursor.value) return
  isLoadingMore.value = true
  const next = await kunFetch<{ items: ActivityItem[]; next_cursor: string }>(
    '/activity/timeline',
    {
      method: 'GET',
      query: {
        limit: 50,
        cursor: cursor.value,
        show_no_resource: settings.showKUNGalgameNoResource
      }
    }
  )
  isLoadingMore.value = false
  if (!next) return
  items.value.push(...next.items)
  cursor.value = next.next_cursor
  hasMore.value = !!next.next_cursor
}

// Auto-load when the sentinel near the page bottom scrolls into view. SSR-safe
// (VueUse no-ops on the server); the "加载更多" button is the manual fallback.
const sentinel = ref<HTMLElement | null>(null)
useIntersectionObserver(
  sentinel,
  ([entry]) => {
    if (entry?.isIntersecting) loadMore()
  },
  { rootMargin: '400px' }
)
</script>

<template>
  <div class="space-y-3">
    <KunHeader
      name="动态时间线"
      description="动态时间线, 展示全站 话题, 回复, Galgame 与社区的最新 Galgame 资源, Galgame 动态, Galgame 讨论, Galgame 评论等"
    />

    <KunNull
      v-if="status !== 'pending' && !items.length"
      description="暂无动态"
    />

    <div v-else class="relative space-y-6">
      <div class="bg-primary/20 absolute top-6 bottom-0 left-4 w-0.5" />

      <div
        v-for="activity in items"
        :key="activity.unique_id"
        class="flex items-center gap-3"
      >
        <KunAvatar v-if="activity.actor" :user="activity.actor" />

        <div class="flex flex-col space-y-2">
          <KunLink
            underline="none"
            color="default"
            :to="activity.link"
            class-name="hover:text-primary block space-x-3 break-all transition-colors"
          >
            <KunText
              class-name="whitespace-normal!"
              :content="markdownToText(activity.content)"
            />
            <KunChip color="primary" size="xs">
              {{ KUN_ACTIVITY_TYPE_TYPE[activity.type] }}
            </KunChip>
          </KunLink>

          <div class="flex items-center space-x-2">
            <span class="text-default-500 text-sm">
              <template v-if="activity.actor"
                >{{ activity.actor.name }} 发布于 </template
              ><KunTime :time="activity.timestamp" />
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Infinite-scroll sentinel + manual fallback / end state -->
    <div v-if="items.length" ref="sentinel" class="flex justify-center pt-1">
      <KunButton
        v-if="hasMore"
        variant="light"
        :loading="isLoadingMore"
        @click="loadMore"
      >
        加载更多
      </KunButton>
      <span v-else class="text-default-400 text-sm">没有更多动态了</span>
    </div>
  </div>
</template>
