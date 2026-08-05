<script setup lang="ts">
import { kunUpdateLogTabItem } from '~/constants/update'

const activeTab = computed(
  () => useRoute().fullPath.split('/').pop() || 'history'
)
</script>

<template>
  <div class="space-y-3">
    <KunHeader
      name="更新日志"
      description="鲲 Galgame 论坛的更新历史与开发待办，记录每一次迭代与计划。"
    />

    <!-- Nav on the left, content on the right (below the header). -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start">
      <div class="shrink-0 sm:w-32">
        <KunTab
          :items="kunUpdateLogTabItem"
          :model-value="activeTab"
          orientation="vertical"
          variant="underlined"
          color="primary"
          full-width
        />
      </div>

      <div class="min-w-0 flex-1">
        <NuxtPage />
      </div>
    </div>
  </div>
</template>
