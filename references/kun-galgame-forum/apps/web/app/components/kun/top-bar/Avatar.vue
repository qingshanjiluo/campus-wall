<script setup lang="ts">
const { id, name, avatar } = storeToRefs(usePersistUserStore())
const { showKUNGalgamePanel, messageStatus } = storeToRefs(
  useTempSettingStore()
)

// Single solid-primary "登录" button replaces the previous (登录 + 注册)
// pair. Click opens the global KunAuthModal (mounted in app.vue) which offers
// both options as OAuth jumps. Pre-L1 there were standalone /login + /register
// pages; both were deleted since the actual auth work is owned by OAuth.
const { open: openAuthModal } = useAuthModal()

// KunPopover stays open on inside-clicks by design (it's a generic overlay).
// For this menu we want item clicks to dismiss it, so we hold a ref and call
// the exposed close() when UserInfo signals an item was activated.
const userMenu = ref<{ close: () => void } | null>(null)

const onKeydown = async (event: KeyboardEvent) => {
  if (event.ctrlKey && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    await navigateTo('/search')
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))

onUnmounted(() => window.removeEventListener('keydown', onKeydown))

const statusClasses = computed(() => {
  if (messageStatus.value === 'admin') {
    return 'bg-danger'
  } else if (messageStatus.value === 'new') {
    return 'bg-secondary'
  } else if (messageStatus.value === 'online') {
    return 'bg-success'
  } else {
    return 'bg-primary'
  }
})
</script>

<template>
  <div class="flex items-center space-x-1 leading-none">
    <KunTooltip text="Ctrl + K 以快速搜索" position="bottom">
      <KunButton
        :is-icon-only="true"
        variant="light"
        color="default"
        size="xl"
        href="/search"
      >
        <KunIcon name="lucide:search" />
      </KunButton>
    </KunTooltip>

    <KunButton
      :is-icon-only="true"
      variant="light"
      color="default"
      size="xl"
      @click="showKUNGalgamePanel = !showKUNGalgamePanel"
    >
      <KunIcon name="lucide:settings" />
    </KunButton>

    <KunPopover ref="userMenu" position="bottom-end" inner-class="p-2 min-w-60">
      <template v-if="id" #trigger>
        <!-- relative inline-flex pins the status dot to the avatar's own box.
             Without it the dot anchored to KunPopover's trigger wrapper, which
             since kun-ui 1.x is an inline-block — its baseline descender gap
             pushed the dot a few px below the avatar's corner. -->
        <div class="relative inline-flex">
          <KunAvatar
            size="lg"
            :is-navigation="false"
            :user="{ id, name, avatar }"
            :disable-floating="true"
          />
          <div
            class="absolute right-0 bottom-0 size-2 rounded-full"
            :class="cn(statusClasses, messageStatus)"
          />
        </div>
      </template>

      <!-- UserInfo is a code-split (Lazy) chunk; on the first menu open it
           downloads, which on a cold/slow connection left the popover blank
           for a beat. UserInfo itself renders from the store (no fetch), so
           the only async is the chunk — show a KunLoading until it resolves. -->
      <Suspense>
        <LazyKunTopBarUserInfo @close="userMenu?.close()" />
        <template #fallback>
          <div class="flex min-h-48 min-w-56 items-center justify-center py-6">
            <KunLoading />
          </div>
        </template>
      </Suspense>
    </KunPopover>

    <template v-if="!id">
      <KunButton size="lg" color="primary" @click="openAuthModal()">
        登录
      </KunButton>
    </template>

    <!-- 萌萌点明细 + 退出登录 modals are NOT mounted here. They self-bind to a
         temp-store flag set by the menu in UserInfo.vue, but their root is a
         teleporting <KunModal>; a <style scoped> parent (this file) can't stamp
         its scope id onto a teleport root → "Extraneous non-props attributes"
         warning. So they live at the non-scoped app.vue root instead. -->
  </div>
</template>

<style  scoped>
.new {
  animation: kun-pulse 1s infinite;
}

.admin {
  animation: kun-pulse 1s infinite;
}

@keyframes kun-pulse {
  0% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.5);
    opacity: 0.5;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}
</style>
