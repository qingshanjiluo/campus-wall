<script setup lang="ts">
import { storeToRefs } from 'pinia'

// Filters are URL-backed (useGalgameFilters / useRouteQuery). The refs
// double as the fetch query — URL key === BE query key, so no remapping.
// useKunFetch watches the query refs, so a chip toggle (which writes the
// URL) re-fetches; browser back/forward re-fetches too.
const {
  page,
  limit,
  type,
  language,
  platform,
  gameType,
  sortField,
  sortOrder,
  releasedFrom,
  releasedTo,
  releasedMonths,
  includeProviders,
  excludeOnlyProviders,
  minRatingCount,
  minRating
} = useGalgameFilters()

// Global "显示没有下载资源的 Galgame" preference (cookie-persisted settings
// store, SSR-safe). Off (default) hides resource-less galgames. Passed in the
// query so toggling it re-fetches.
const { showKUNGalgameNoResource } = storeToRefs(usePersistSettingsStore())

const route = useRoute()

const { data, status, refresh } = await useKunFetch<{
  galgames: GalgameCard[]
  total: number
}>(`/galgame`, {
  method: 'GET',
  query: {
    page,
    limit,
    type,
    language,
    platform,
    game_type: gameType,
    sort_field: sortField,
    sort_order: sortOrder,
    released_from: releasedFrom,
    released_to: releasedTo,
    released_months: releasedMonths,
    include_providers: includeProviders,
    exclude_only_providers: excludeOnlyProviders,
    min_rating_count: minRatingCount,
    min_rating: minRating,
    show_no_resource: showKUNGalgameNoResource
  },
  // Don't auto-refetch on every query-ref change: clicking a card navigates to
  // /galgame/:id, which resets these URL-backed filters to their defaults and
  // would otherwise fire a wasted default-params fetch right before unmount.
  // Refetch manually instead, and ONLY while still on the list route.
  watch: false
})

const listPath = route.path
watch([() => route.fullPath, showKUNGalgameNoResource], () => {
  if (route.path === listPath) {
    refresh()
  }
})
</script>

<template>
  <div class="flex flex-col gap-3">
    <template v-if="data">
      <div class="z-10">
        <KunHeader name="Galgame 资源资料库">
          <template #endContent>
            <GalgameCardNav :is-show-advanced="true" />
          </template>

          <template #description>
            <p class="text-default-500">
              Galgame 资源页面, 提供各类 Galgame 下载。我们不是资源的提供者,
              我们只是资源的指路人。
            </p>
          </template>
        </KunHeader>
      </div>

      <KunLoading :loading="status === 'pending'">
        <GalgameCard v-if="data.galgames" :galgames="data.galgames" />
      </KunLoading>

      <KunCard
        :is-hoverable="false"
        :is-transparent="false"
        content-class="gap-3"
      >
        <KunPagination
          v-model:current-page="page"
          :total-page="Math.ceil(data.total / limit)"
          :is-loading="status === 'pending'"
        />
      </KunCard>
    </template>
  </div>
</template>
