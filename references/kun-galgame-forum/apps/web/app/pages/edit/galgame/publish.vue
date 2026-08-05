<script setup lang="ts">
definePageMeta({
  middleware: ['auth']
})

useKunDisableSeo('发布 Galgame')
</script>

<template>
  <EditGalgameWizard />
</template>
