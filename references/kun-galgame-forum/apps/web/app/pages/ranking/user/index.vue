<script setup lang="ts">
import { rankingPageMetaData } from '~/constants/ranking'

useKunSeoMeta({
  title: rankingPageMetaData['user']?.title,
  description: rankingPageMetaData['user']?.description
})
</script>

<template>
  <RankingUser />
</template>
