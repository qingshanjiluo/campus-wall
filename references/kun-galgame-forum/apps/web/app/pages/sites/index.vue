<script setup lang="ts">
import { kunSubSites } from '~/constants/layout'

useKunSeoMeta({
  title: '子网站',
  description: `${kungal.titleShort} 的子网站导航, 汇总 ${kunSubSites
    .map((site) => site.short)
    .join('、')} 等 Galgame 相关的独立站点与工具。`
})
</script>

<template>
  <SitesContainer />
</template>
