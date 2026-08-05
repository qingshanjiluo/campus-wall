<script setup lang="ts">
definePageMeta({
  middleware: ['auth', 'prevent']
})

useKunDisableSeo('发布 Galgame 工具')
</script>

<template>
  <EditToolset />
</template>
