<script setup lang="ts">
import { navItems } from './items'

const { keywords } = storeToRefs(useTempSearchStore())

// Backend (Go SearchHandler) wraps every type in `{items, total}` via
// response.Paginated. Match that shape verbatim.
interface SearchPage {
  items: SearchResult[]
  total: number
}

const results = ref<SearchResult[]>([])
const total = ref(0)
const isLoading = ref(false)
// Search type lives in the URL (?type=) so it survives back/forward + sharing.
const activeType = useTabQuery('topic', 'type')
const pageData = reactive({
  page: 1,
  limit: 12
})

// Derive load-complete from total instead of counting pages — works for the
// last page being non-full and for total being 0 (no results).
const isLoadComplete = computed(
  () => total.value > 0 && results.value.length >= total.value
)

const searchQuery = async (searchType?: string): Promise<SearchPage> => {
  isLoading.value = true
  const result = await kunFetch<SearchPage>('/search', {
    method: 'GET',
    query: {
      keywords: keywords.value,
      type: searchType || activeType.value,
      ...pageData
    }
  })
  isLoading.value = false
  return result ?? { items: [], total: 0 }
}

const handleSetType = async (value: SearchType) => {
  activeType.value = value
  pageData.page = 1
  results.value = []
  total.value = 0

  if (keywords.value) {
    const page = await searchQuery(value)
    results.value = page.items
    total.value = page.total
  } else {
    isLoading.value = false
  }
}

// `immediate` so returning to /search (no longer keepalive) re-runs the persisted
// search on mount and restores the results, instead of showing an empty page.
watch(
  () => keywords.value,
  async () => {
    pageData.page = 1

    if (keywords.value) {
      const page = await searchQuery()
      results.value = page.items
      total.value = page.total
    } else {
      results.value = []
      total.value = 0
      isLoading.value = false
    }
  },
  { immediate: true }
)

const handleLoadMore = async () => {
  pageData.page++
  const page = await searchQuery()
  results.value = results.value.concat(page.items)
  total.value = page.total
}
</script>

<template>
  <div class="min-h-[calc(100dvh-6rem)] space-y-6">
    <KunHeader
      name="搜索"
      description="您可以在本页面搜索本论坛的所有话题, Galgame, 用户, 回复, 评论。"
    >
      <template #endContent>
        <div class="text-default-500">
          当前的搜索会一并搜索 NSFW 内容, 如果您要按照 Galgame 厂商 / 会社 /
          标签搜索, 或者需要 <KunLink to="/galgame/tag">多标签搜索</KunLink> ,
          请前往
          <KunLink to="/galgame/official"> Galgame 会社资料库 </KunLink>
          或者
          <KunLink to="/galgame/tag"> Galgame 标签资料库 </KunLink>
          的页面进行搜索。
        </div>
      </template>
    </KunHeader>
    <KunTab
      :items="navItems"
      :model-value="activeType"
      @update:model-value="(value) => handleSetType(value as SearchType)"
      size="sm"
    />

    <SearchBox />

    <SearchHistory v-if="!keywords" />

    <SearchResult
      :results="results"
      :type="activeType as SearchType"
      v-if="results.length"
    />

    <KunDivider v-if="results.length >= 12">
      <slot />
      <KunButton
        variant="flat"
        :loading="isLoading"
        :disabled="isLoading || isLoadComplete"
        @click="handleLoadMore"
      >
        加载更多
      </KunButton>
      <span v-if="isLoadComplete">被榨干了呜呜呜呜呜, 一滴也不剩了</span>
    </KunDivider>

    <KunNull
      v-if="!results.length && keywords && !isLoading"
      description="杂鱼杂鱼杂鱼~什么也没有搜索到"
    />

    <KunLoading v-if="isLoading" />
  </div>
</template>
