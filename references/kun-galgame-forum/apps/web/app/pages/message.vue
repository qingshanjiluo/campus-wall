<script setup lang="ts">
definePageMeta({
  middleware: 'auth',
  // The /message subpages use overlayscrollbars (<KunOverlayScroll>), which
  // rebuilds its subtree's DOM outside Vue's vdom. With the global `out-in`
  // page transition (nuxt.config app.pageTransition), client-side route
  // switches between message subpages went blank — the leaving page's
  // OS-managed DOM and the transition's teardown race — while a hard refresh
  // (SSR) was always fine. A chat UI doesn't need a page transition, so disable
  // it for this page and (below) its nested subpages.
  pageTransition: false
})

const { messageStatus } = storeToRefs(useTempSettingStore())

useHead({ title: `消息 - ${kungal.titleShort}` })

onMounted(() => (messageStatus.value = 'online'))
</script>

<template>
  <div class="flex h-[calc(100dvh-120px)] flex-row sm:gap-4">
    <!-- sm:gap-4 keeps the content off the aside's divider on desktop; on mobile
         only one pane shows, so the gap is a no-op there. -->
    <MessageAsideContainer />
    <NuxtPage :transition="false" />
  </div>
</template>
