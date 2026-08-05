<script setup lang="ts">
// Bare /user/:id → the 动态 landing tab. /user/:id is the canonical profile URL
// (avatars, @mentions, cards and SEO author links all point here); this redirect
// makes 动态 the profile homepage without every link having to spell out
// /activity. Resolved at routing time (SSR-safe, no flash).
definePageMeta({
  redirect: (to) => `/user/${to.params.id}/activity`
})
</script>

<template>
  <div />
</template>
