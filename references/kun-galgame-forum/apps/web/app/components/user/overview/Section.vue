<script setup lang="ts">
// One block of the 动态 overview: a titled section showing a section's latest
// few items, with a 查看全部 link to its full tab. Pass `items` for the compact
// text-row layout (话题 / 资源 / 回复 / 评论), or use the default slot to drop in a
// rich card grid (Galgame / 评分).
defineProps<{
  title: string
  icon: string
  viewAllHref: string
  items?: { text: string; time: Date | string | number; href: string }[]
}>()
</script>

<template>
  <section class="space-y-3">
    <div class="flex items-center justify-between gap-2">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <KunIcon :name="icon" class="text-primary" />
        {{ title }}
      </h2>
      <KunLink :to="viewAllHref" class="text-default-500 shrink-0 text-sm">
        查看全部
        <KunIcon name="lucide:chevron-right" class="align-middle" />
      </KunLink>
    </div>

    <div v-if="items" class="flex flex-col gap-2">
      <KunCard v-for="(item, i) in items" :key="i" :href="item.href">
        <div class="line-clamp-2 text-sm break-words">{{ item.text }}</div>
        <div class="text-default-500 mt-1 text-xs">
          <KunTime :time="item.time" type="date" show-year />
        </div>
      </KunCard>
    </div>

    <slot v-else />
  </section>
</template>
