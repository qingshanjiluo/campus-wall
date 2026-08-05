<script setup lang="ts">
import {
  spoilerOptions,
  playStatusOptions,
  typeOptions,
  sortFieldOptions
} from './_sort'
const params = reactive({
  page: 1,
  limit: 24,
  sort_field: 'time',
  sort_order: 'desc',
  spoiler_level: 'all',
  play_status: 'all',
  galgame_type: 'all'
})

const { data, status } = await useKunFetch<{
  rating_data: GalgameRatingCard[]
  total: number
}>(`/galgame-rating/all`, {
  method: 'GET',
  query: params
})
</script>

<template>
  <div class="space-y-3">
    <div class="space-y-2">
      <KunHeader name="Galgame 评分列表">
        <template #description>
          <p class="text-default-500">
            浏览所有用户对所有 Galgame 的评分与短评，支持按 Galgame 剧透等级,
            Galgame 游玩状态, Galgame 游戏类型筛选, 并按时间, Galgame 热度或
            Galgame 评分进行排序。
          </p>
        </template>
      </KunHeader>

      <div
        class="flex w-full shrink-0 flex-wrap items-center justify-between gap-3 rounded-lg border border-transparent sm:flex-nowrap"
      >
        <div class="grid w-full grid-cols-2 gap-3 lg:grid-cols-4">
          <KunSelect v-model="params.spoiler_level" :options="spoilerOptions" />
          <KunSelect v-model="params.play_status" :options="playStatusOptions" />
          <KunSelect v-model="params.galgame_type" :options="typeOptions" />
          <KunSelect v-model="params.sort_field" :options="sortFieldOptions" />
        </div>

        <div class="flex items-center gap-2">
          <KunButton
            :is-icon-only="true"
            :variant="params.sort_order === 'desc' ? 'flat' : 'light'"
            size="lg"
            @click="params.sort_order = 'desc'"
          >
            <KunIcon class="text-inherit" name="lucide:arrow-down" />
          </KunButton>

          <KunButton
            :is-icon-only="true"
            :variant="params.sort_order === 'asc' ? 'flat' : 'light'"
            size="lg"
            @click="params.sort_order = 'asc'"
          >
            <KunIcon class="text-inherit" name="lucide:arrow-up" />
          </KunButton>
        </div>
      </div>
    </div>

    <GalgameRatingCard v-if="data" :ratings="data.rating_data" />

    <KunPagination
      v-if="(data?.total || 0) > params.limit"
      v-model:current-page="params.page"
      :total-page="Math.ceil((data?.total || 0) / params.limit)"
      :is-loading="status === 'pending'"
    />
  </div>
</template>
