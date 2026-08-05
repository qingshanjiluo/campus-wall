<script setup lang="ts">
definePageMeta({
  middleware: 'auth'
})

useKunDisableSeo('发布话题')
</script>

<template>
  <EditTopicLayout />
</template>
