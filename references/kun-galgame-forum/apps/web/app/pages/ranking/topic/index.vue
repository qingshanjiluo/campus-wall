<script setup lang="ts">
import { rankingPageMetaData } from '~/constants/ranking'

useKunSeoMeta({
  title: rankingPageMetaData['topic']?.title,
  description: rankingPageMetaData['topic']?.description
})
</script>

<template>
  <RankingTopic />
</template>
