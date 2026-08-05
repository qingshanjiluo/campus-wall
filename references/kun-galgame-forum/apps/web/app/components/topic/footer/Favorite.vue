<script setup lang="ts">
// Topic 收藏 — a thin binding of the shared FavoriteToggle to the topic endpoint.
const props = defineProps<{
  topicId: number
  favoriteCount: number
  isFavorite: boolean
}>()
</script>

<template>
  <FavoriteToggle
    :favorited="props.isFavorite"
    :count="props.favoriteCount"
    :endpoint="`/topic/${props.topicId}/favorite`"
    :messages="[10230, 10231]"
  />
</template>
