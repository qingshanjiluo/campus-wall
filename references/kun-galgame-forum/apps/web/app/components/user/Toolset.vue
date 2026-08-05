<script setup lang="ts">
// A user's authored Galgame 工具 (toolsets) — the 工具 chip of the Galgame group.
// Reuses the shared ToolsetCard grid; data comes from GET /user/:id/toolsets
// (the toolset list scoped to this author).
const props = defineProps<{
  userId: number
}>()

const pageData = reactive({
  page: 1,
  limit: 24
})

const { data, status } = await useKunFetch<{
  items: ToolsetCard[]
  total: number
}>(`/user/${props.userId}/toolsets`, { query: pageData })
</script>

<template>
  <div class="space-y-3">
    <div v-if="data && data.items.length" class="space-y-3">
      <ToolsetCard :items="data.items" />

      <KunPagination
        v-if="data.total > pageData.limit"
        v-model:current-page="pageData.page"
        :total-page="Math.ceil(data.total / pageData.limit)"
        :is-loading="status === 'pending'"
      />
    </div>

    <KunNull v-if="data && !data.items.length" description="暂无工具" />
  </div>
</template>
