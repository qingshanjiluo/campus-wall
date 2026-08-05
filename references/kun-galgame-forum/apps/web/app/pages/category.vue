<script setup lang="ts">
import { KUN_TOPIC_CATEGORY } from '~/constants/topic'
import { KUN_CATEGORY_DESCRIPTION_MAP } from '~/constants/category'

const activeTab = computed(
  () => useRoute().fullPath.split('/').pop() || 'galgame'
)
</script>

<template>
  <div class="h-full space-y-3 justify-start">
    <KunHeader
      :name="KUN_TOPIC_CATEGORY[activeTab]"
      :description="KUN_CATEGORY_DESCRIPTION_MAP[activeTab]"
    />

    <NuxtPage />
  </div>
</template>
