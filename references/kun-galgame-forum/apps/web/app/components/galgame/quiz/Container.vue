<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import type { KunTabItem } from '@kungal/ui-vue'
import {
  quizCategoryOptions,
  quizTypeOptions,
  quizDifficultyOptions,
  quizSortFieldOptions
} from './_filters'

const route = useRoute()
const userStore = usePersistUserStore()
const isLoggedIn = computed(() => !!userStore.id)

// Browse state lives entirely in the URL query — the project-wide idiom (see
// topic/Container.vue and useGalgameFilters). It makes the filtered/paged view
// shareable + refresh-stable, and lets a quiz's 返回题库 restore it with a plain
// browser-history round-trip (router.back in Play.vue). `mode:'replace'` keeps
// filter tweaks out of the history stack; values equal to the default are
// dropped, so an unfiltered browse stays a clean /galgame-quiz.
const opts = { mode: 'replace' as const }
const page = useRouteQuery('page', 1, { ...opts, transform: Number })
const tab = useRouteQuery<'all' | 'mine'>('tab', 'all', opts)
const category = useRouteQuery<string>('category', 'all', opts)
const type = useRouteQuery<string>('type', 'all', opts)
const difficulty = useRouteQuery('difficulty', 0, { ...opts, transform: Number })
const sortField = useRouteQuery<string>('sort_field', 'update_time', opts)
const sortOrder = useRouteQuery<'asc' | 'desc'>('sort_order', 'desc', opts)
const limit = 50

// `mine` is self-only (login-gated); a shared ?tab=mine opened while logged out
// falls back to the public list instead of hitting the auth-required endpoint.
const activeTab = computed(() => (isLoggedIn.value ? tab.value : 'all'))
const requestUrl = computed(() =>
  activeTab.value === 'mine' ? '/galgame-quiz/mine/answered' : '/galgame-quiz/all'
)

const { data, status, refresh } = await useKunFetch<QuizListPage>(requestUrl, {
  method: 'GET',
  query: {
    page,
    limit,
    sort_field: sortField,
    sort_order: sortOrder,
    category,
    type,
    difficulty
  },
  // Don't auto-refetch on every query-ref change: opening a quiz navigates to
  // /galgame-quiz/:id, which resets the URL-backed refs and would fire a wasted
  // fetch right before unmount. Refetch manually, ONLY while still on this list.
  watch: false
})

const listPath = route.path
watch(
  () => route.fullPath,
  () => {
    if (route.path !== listPath) return
    refresh()
    if (import.meta.client) window.scrollTo({ top: 0, behavior: 'smooth' })
  }
)

// Any filter / tab change returns to page 1. useRouteQuery batches the writes
// within a tick, so the page reset and the filter write land as one route update
// → one refetch (no self-clobber, no double fetch).
watch([category, type, difficulty, sortField, sortOrder, tab], () => {
  page.value = 1
})

const tabItems: KunTabItem[] = [
  { value: 'all', textValue: '全部题库', icon: 'lucide:library-big' },
  { value: 'mine', textValue: '我的答题', icon: 'lucide:history' }
]
const onTab = (v: string) => {
  tab.value = v as 'all' | 'mine'
}
const setSortOrder = (order: 'asc' | 'desc') => {
  sortOrder.value = order
}

const showPublish = ref(false)
const openPublish = () => {
  if (!requireLogin()) return
  showPublish.value = true
}
const onPublished = () => {
  tab.value = 'all'
  page.value = 1
  refresh()
}
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-2">
      <KunHeader name="Galgame 题库">
        <template #description>
          <p class="text-default-500">
            由 Galgame 爱好者共同建设的题库, 支持单选、多选、判断、填空、问答等题型。答对可获得萌萌点, 出题合格同样有奖励。
          </p>
        </template>
      </KunHeader>

      <div class="flex items-center gap-2">
        <KunTab
          v-if="isLoggedIn"
          :model-value="activeTab"
          :items="tabItems"
          variant="light"
          color="primary"
          @update:model-value="onTab"
        />
        <KunButton class="ml-auto shrink-0" @click="openPublish">
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:plus" />
            出题
          </span>
        </KunButton>
      </div>

      <div
        v-if="activeTab === 'all'"
        class="flex w-full shrink-0 flex-wrap items-center justify-between gap-3 sm:flex-nowrap"
      >
        <div class="grid w-full grid-cols-2 gap-3 lg:grid-cols-4">
          <KunSelect v-model="category" :options="quizCategoryOptions" />
          <KunSelect v-model="type" :options="quizTypeOptions" />
          <KunSelect v-model="difficulty" :options="quizDifficultyOptions" />
          <KunSelect v-model="sortField" :options="quizSortFieldOptions" />
        </div>

        <div class="flex items-center gap-2">
          <KunButton
            :is-icon-only="true"
            :variant="sortOrder === 'desc' ? 'flat' : 'light'"
            size="md"
            @click="setSortOrder('desc')"
          >
            <KunIcon class="text-inherit" name="lucide:arrow-down" />
          </KunButton>
          <KunButton
            :is-icon-only="true"
            :variant="sortOrder === 'asc' ? 'flat' : 'light'"
            size="md"
            @click="setSortOrder('asc')"
          >
            <KunIcon class="text-inherit" name="lucide:arrow-up" />
          </KunButton>
        </div>
      </div>
    </div>

    <GalgameQuizList
      v-if="data && data.quiz_data.length"
      :quizzes="data.quiz_data"
    />
    <KunNull
      v-else-if="status !== 'pending'"
      :description="
        activeTab === 'mine'
          ? '你还没有作答过任何题目'
          : '还没有人出题, 快来出第一题吧'
      "
    />

    <KunPagination
      v-if="(data?.total || 0) > limit"
      v-model:current-page="page"
      :total-page="Math.ceil((data?.total || 0) / limit)"
      :is-loading="status === 'pending'"
    />

    <GalgameQuizPublish v-model="showPublish" @on-published="onPublished" />
  </div>
</template>
