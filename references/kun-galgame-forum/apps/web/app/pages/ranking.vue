<script setup lang="ts">
import {
  topicSortItem,
  galgameSortItem,
  userSortItem,
  rankingPageTabs,
  rankingPageMetaData
} from '~/constants/ranking'
import {
  topicRankingPageData,
  galgameRankingPageData,
  userRankingPageData
} from '~/components/ranking/pageData'
import type { KunSelectOption } from '@kungal/ui-vue'

// The tab is the last path segment, but only when it NAMES a tab. Bare
// /ranking yields "ranking", which is not a key of rankingPageMetaData — the
// `?? 'user'` below never fired for it (the segment is present, just wrong) and
// the header read .title off undefined, crashing SSR with a 500. Match against
// the tab list so any non-tab segment lands on the default instead.
const rankingTabs = new Set(rankingPageTabs.map((tab) => tab.value))

const activeTab = computed(() => {
  const segment = useRoute().path.split('/').filter(Boolean).pop()
  return segment && rankingTabs.has(segment) ? segment : 'user'
})

const currentSortItems = computed(() => {
  switch (activeTab.value) {
    case 'topic':
      return topicSortItem
    case 'galgame':
      return galgameSortItem
    case 'user':
    default:
      return userSortItem
  }
})

const sortOptions = computed(() => {
  return currentSortItems.value.map((item) => ({
    value: item.sortField,
    label: item.label,
    icon: item.icon
  }))
})
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-3">
      <KunHeader
        :name="rankingPageMetaData[activeTab]!.title"
        :description="rankingPageMetaData[activeTab]!.description"
      />

      <div class="flex items-center justify-between">
        <KunTab
          :model-value="activeTab"
          :items="rankingPageTabs"
          variant="underlined"
          color="primary"
        />

        <div class="w-48">
          <KunSelect
            v-if="activeTab === 'topic'"
            v-model="topicRankingPageData.sort_field"
            :options="
              sortOptions as KunSelectOption<
                typeof topicRankingPageData.sort_field
              >[]
            "
          />
          <KunSelect
            v-if="activeTab === 'galgame'"
            v-model="galgameRankingPageData.sort_field"
            :options="
              sortOptions as KunSelectOption<
                typeof galgameRankingPageData.sort_field
              >[]
            "
          />
          <KunSelect
            v-if="activeTab === 'user'"
            v-model="userRankingPageData.sort_field"
            :options="
              sortOptions as KunSelectOption<
                typeof userRankingPageData.sort_field
              >[]
            "
          />
        </div>
      </div>
    </div>

    <NuxtPage />
  </div>
</template>
