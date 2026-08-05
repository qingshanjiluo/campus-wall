<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'

const tempReplyStore = useTempReplyStore()
const { isEdit, isReplyRewriting } = storeToRefs(tempReplyStore)

// Mobile gets a bottom KunDrawer; desktop keeps the original fixed panel.
// `isEdit` drives both (PanelBtn flips it false on publish / rewrite / cancel),
// and a KunDrawer dismiss also flips it via v-model. Gate the shell switch
// behind mount: useMediaQuery is true synchronously on a mobile client, so
// without this the first client render (drawer) would mismatch SSR (desktop).
// Both shells are empty until isEdit, so the post-mount flip is invisible.
const isMobileQuery = useMediaQuery('(max-width: 767px)')
const mounted = ref(false)
onMounted(() => (mounted.value = true))
const isMobile = computed(() => mounted.value && isMobileQuery.value)

// Backdrop-dismissing the drawer mid-rewrite must drop the rewrite buffer,
// otherwise the next fresh reply would reopen pre-filled with the old content.
const handleDrawerClose = () => {
  if (isReplyRewriting.value) {
    tempReplyStore.resetRewriteReplyData()
  }
}
</script>

<template>
  <KunDrawer
    v-if="isMobile"
    v-model="isEdit"
    placement="bottom"
    size="lg"
    title="发表回复"
    @close="handleDrawerClose"
  >
    <TopicReplyPanelBody />
  </KunDrawer>

  <Teleport v-else to="body" :disabled="!isEdit">
    <Transition
      enter-active-class="animate-fadeInUp"
      leave-active-class="animate-fadeOutDown"
    >
      <div
        class="fixed bottom-0 z-100 flex max-h-[80%] w-full flex-col items-center"
        v-if="isEdit"
      >
        <div
          class="kun-reply-panel bg-content1 border-default/20 scrollbar-hide w-full max-w-4xl space-y-2 overflow-scroll rounded-t-lg border p-3"
        >
          <TopicReplyPanelBody />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
