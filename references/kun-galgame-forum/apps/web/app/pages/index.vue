<script setup lang="ts">
// keepalive: returning from a card restores the feed (tab + scroll + loaded
// items) instead of re-mounting to the default tab at the top.
definePageMeta({ keepalive: true })

useKunSeoMeta({
  title: '主页',
  description: kungal.description
})
</script>

<template>
  <HomeContainer />
</template>
