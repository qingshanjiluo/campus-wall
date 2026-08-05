<script setup lang="ts">
definePageMeta({
  middleware: ['auth']
})

useKunDisableSeo('我的 Galgame 提交')
</script>

<template>
  <EditGalgameMine />
</template>
