<script setup lang="ts">
// "已静音的消息" aside entry. Muting suppresses badges, so this shows a neutral
// total (not an unread / red-dot count) and links to the muted view. Fetched
// client-only + lazy, mirroring GalgameItem — the aside shouldn't pay for it on SSR.
const { data } = useKunFetch<MessageList>('/message/muted', {
  query: { page: 1, limit: 1, sort_order: 'desc' },
  server: false,
  lazy: true
})
const total = computed(() => data.value?.total ?? 0)
</script>

<template>
  <KunLink
    color="default"
    underline="none"
    class-name="hover:bg-primary/20 flex cursor-pointer flex-nowrap gap-3 rounded-lg p-2 transition-colors hover:opacity-80"
    to="/message/muted"
  >
    <div
      class="bg-default-100 flex h-12 w-12 shrink-0 items-center justify-center rounded-full"
    >
      <KunIcon name="lucide:bell-off" class="text-default-500 text-xl" />
    </div>
    <div class="flex w-full flex-col justify-center">
      <span class="font-bold">已静音的消息</span>
      <div class="flex items-center justify-between text-sm">
        <span class="text-default-500 line-clamp-1">已被你静音的通知</span>
        <KunChip v-if="total" color="default" class-name="whitespace-nowrap">
          {{ total }}
        </KunChip>
      </div>
    </div>
  </KunLink>
</template>
