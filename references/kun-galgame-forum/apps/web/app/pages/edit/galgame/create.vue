<script setup lang="ts">
definePageMeta({
  middleware: ['auth', 'prevent']
})

useKunDisableSeo('发布 Galgame')
</script>

<template>
  <EditGalgame />
</template>
