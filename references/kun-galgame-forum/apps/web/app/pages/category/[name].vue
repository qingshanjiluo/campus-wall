<script setup lang="ts">
import { KUN_TOPIC_CATEGORY } from '~/constants/topic'
import { KUN_CATEGORY_DESCRIPTION_MAP } from '~/constants/category'

const route = useRoute()

const categoryName = computed(() => {
  return (route.params as { name: string }).name
})

// BE returns `[]SectionStat` (see apps/api/internal/section/dto). The FE
// type name used to be `CategorySection` which never existed — the right
// type is `CategorySectionStats` (already imported by CategoryContainer
// for its `sections` prop).
const { data } = await useKunFetch<CategorySectionStats[]>('/category', {
  query: { category: categoryName }
})

useKunSeoMeta({
  title: KUN_TOPIC_CATEGORY[categoryName.value],
  description: KUN_CATEGORY_DESCRIPTION_MAP[categoryName.value]
})
</script>

<template>
  <CategoryContainer
    v-if="data"
    :sections="data"
    :category-name="categoryName"
  />
</template>
