<script setup lang="ts">
import { rankingPageMetaData } from '~/constants/ranking'

useKunSeoMeta({
  title: rankingPageMetaData['galgame']?.title,
  description: rankingPageMetaData['galgame']?.description
})
</script>

<template>
  <RankingGalgame />
</template>
