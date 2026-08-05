<script setup lang="ts">
const routeName = computed(() => useRoute().name)

// SSR these so the aside renders with its items on first paint instead of
// flashing empty. useKunFetch forwards the session cookie on SSR (see kunFetch
// onRequest), so the authed nav fetch resolves server-side.
const { data: systemNav } = useKunFetch<ChatMessageAsideItem[]>(
  '/message/nav/system'
)
const { data: contactNav } = useKunFetch<ChatMessageAsideItem[]>(
  '/message/nav/contact'
)

// Render the chat list straight off this per-request fetch (exactly like
// `system` below), NOT through a watch into the shared asideItems store. A
// watch can't populate the list during SSR: on the server, watch callbacks run
// only for the immediate tick — before the fetch resolves — and post-resolve
// reactive changes don't fire, so the list rendered empty server-side and only
// filled in after client hydration. Reading the fetch directly makes it SSR.
//
// The `as` casts pin the element type: useKunFetch's shared transform unwraps
// every endpoint's `data`, so TS widens it toward `{}` (and Nuxt's generated
// useFetch types can transiently resolve to `{}` mid-regeneration), which
// otherwise flags `system[0]` / the `room` loop as un-indexable.
const system = computed(() => systemNav.value as ChatMessageAsideItem[] | null)
const contact = computed(
  () => (contactNav.value as ChatMessageAsideItem[] | null) ?? []
)
</script>

<template>
  <aside
    :class="
      cn(
        'scrollbar-hide border-default-200/60 flex w-full shrink-0 flex-col space-y-3 overflow-y-auto pr-0 sm:w-88 sm:border-r sm:pr-3',
        routeName !== 'message' ? 'hidden sm:flex' : ''
      )
    "
  >
    <h2 class="px-2 text-2xl">消息</h2>

    <KunDivider />

    <MessageAsideSystemItem v-if="system" title="通知" :data="system[0]!" />

    <MessageAsideMutedItem />

    <MessageAsideSystemItem v-if="system" title="系统消息" :data="system[1]!">
      <template #system>
        <span v-if="!system[1]!.unread_count" class="zako">杂鱼~♡</span>
        <span v-if="system[1]!.unread_count" class="new">
          {{ `「 新消息 」` }}
        </span>
      </template>
    </MessageAsideSystemItem>

    <MessageAsideItem
      v-for="(room, index) in contact"
      :key="index"
      :room="room"
    />

    <div class="block p-2 sm:hidden">
      <h2 class="text-lg">提示</h2>
      <div>本消息系统尚在开发中, 但是功能应该足够用</div>
      <div>如果您有任何问题, 请查看这个话题</div>
      <KunLink
        to="https://www.kungal.com/topic/1650"
        target="_blank"
        class="text-primary underline"
      >
        [公告] 有关论坛消息系统的说明
      </KunLink>
    </div>
  </aside>
</template>
