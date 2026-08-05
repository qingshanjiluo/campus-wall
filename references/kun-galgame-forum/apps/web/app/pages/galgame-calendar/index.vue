<script setup lang="ts">
const description =
  'Galgame 发售月历, 按发售月份浏览即将与已发售的 Galgame, 含本月新作、年内待定与发售日期未定。数据来自 Galgame 资料库。'

useKunSeoMeta({
  title: 'Galgame 发售月历',
  description
})
</script>

<template>
  <GalgameCalendarContainer />
</template>
