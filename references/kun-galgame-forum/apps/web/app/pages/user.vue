<script setup lang="ts">
// Key by the :id param so navigating between two profiles client-side
// (e.g. someone else's → your own via the user menu) REMOUNTS this page —
// otherwise the parent route component is reused and the one-shot setup fetch
// below keeps showing the previous user until a hard refresh. Keyed on id (not
// path) so switching subpages of the same user (info → topic → …) doesn't
// remount needlessly.
import {
  kunUserMainNav,
  userSegmentGroup,
  userSegmentHref,
  userTopicGroupOptions,
  userGalgameGroupOptions
} from '~/constants/user'

definePageMeta({ key: (route) => (route.params as { id: string }).id })

const route = useRoute()

const userId = computed(() => {
  return parseInt((route.params as { id: string }).id)
})

const { data } = await useKunFetch<UserInfo>(`/user/${userId.value}`)

// The left rail / pill sub-nav highlights the instant it's clicked, but each
// sub-page remounts + blocks on its own `await useKunFetch` — so the OLD tab's
// content lingers until the new data resolves. Dim the content area during that
// navigation so the switch reads as loading-then-updating.
//
// We drive it off the RAW page:loading hooks, not useLoadingIndicator().isLoading
// — that one is throttled (~200ms) to avoid flashing the top bar, so it never
// turns true on a quick tab switch (which is why the dim looked dead).
//
// A fast/cached main-rail target fires start→end in the SAME tick, so the dim
// class is added+removed before it can paint. Hold it for a minimum window from
// nav start: fast switches still show a brief cue, while a genuinely slow load
// keeps dimming until page:loading:end (past the minimum) — no early flicker.
const isNavLoading = ref(false)
const nuxtApp = useNuxtApp()
const MIN_VISIBLE_MS = 280
let navStartedAt = 0
let clearTimer: ReturnType<typeof setTimeout> | null = null
const stopNavHooks = [
  nuxtApp.hook('page:loading:start', () => {
    if (clearTimer) {
      clearTimeout(clearTimer)
      clearTimer = null
    }
    navStartedAt = Date.now()
    isNavLoading.value = true
  }),
  nuxtApp.hook('page:loading:end', () => {
    const wait = Math.max(0, MIN_VISIBLE_MS - (Date.now() - navStartedAt))
    clearTimer = setTimeout(() => {
      isNavLoading.value = false
    }, wait)
  })
]
onScopeDispose(() => {
  stopNavHooks.forEach((stop) => stop())
  if (clearTimer) {
    clearTimeout(clearTimer)
  }
})

// Owner = the logged-in viewer is on their OWN profile (id match, never a role);
// gates the owner-only 设置 tab.
const { id: storeUid } = storeToRefs(usePersistUserStore())
const isOwner = computed(() => !!storeUid.value && userId.value === storeUid.value)

// The raw active URL segment (/user/:id/<seg>/…), defaulting to 动态 for the bare
// id. The MAIN nav highlights by GROUP (topic/reply/comment → 话题, galgame/
// rating/resource → Galgame); the pill sub-nav highlights by the exact segment.
const activeSegment = computed(() => {
  const m = route.path.match(/^\/user\/\d+\/([^/]+)/)
  return m ? m[1]! : 'activity'
})
const activeGroup = computed(() => userSegmentGroup(activeSegment.value))
const goToSegment = (seg: string) =>
  navigateTo(userSegmentHref(userId.value, seg))

// Banned profiles get a stripped {id, name, status: 1} payload from
// the BE — there's no `'banned'` sentinel string, so the previous
// `data === 'banned'` branch was dead. status !== 0 is the canonical
// "not in good standing" signal.
const isBanned = computed(() => data.value && data.value.status !== 0)

if (isBanned.value) {
  useKunDisableSeo('该用户已被封禁')
} else if (data.value) {
  useKunSeoMeta({
    title: data.value.name,
    description: data.value.bio
  })
} else {
  useKunDisableSeo('未找到该用户')
}
</script>

<template>
  <div class="space-y-4">
    <!-- Single REAL root box (NOT `display: contents`), and this comment lives
         INSIDE it — a comment BEFORE the root is itself a second root node and
         trips Nuxt's "does not have a single root node" warning. The profile is a
         full-width header + tab rail + active sub-tab; the document scrolls. -->
    <template v-if="!isBanned">
      <template v-if="data">
        <UserProfileHeader :user="data" />

        <!-- 左下 = the tab rail, 右下 = the content area. Desktop: a sticky
             vertical rail beside the content; mobile: a horizontal scrollable
             strip stacked on top (the sm:hidden rail takes no grid track). -->
        <div class="grid grid-cols-1 items-start gap-4 sm:grid-cols-[auto_minmax(0,1fr)]">
          <!-- Mobile: a solid (flat-filled) KunTab strip — more prominent than
               the light inner sub-tabs. Only 5 top-level tabs, so it fits;
               `scrollable` is a fallback on very narrow screens. Highlights by
               group (topic/reply/comment → 话题, etc.). -->
          <div class="sm:hidden">
            <KunTab
              :items="kunUserMainNav(data.id, isOwner)"
              :model-value="activeGroup"
              variant="solid"
              color="primary"
              size="sm"
              scrollable
            />
          </div>

          <!-- top-36 (144px), not flush at top-[7.5rem]: the collapsed mini
               header bar is fixed at top-20 and ends ~120px down, so this leaves
               a ~24px breathing gap between that bar and the sticky rail. -->
          <div class="hidden self-start sm:sticky sm:top-36 sm:block">
            <KunTab
              :items="kunUserMainNav(data.id, isOwner)"
              :model-value="activeGroup"
              orientation="vertical"
              variant="underlined"
              color="primary"
            />
          </div>

          <!-- min-height keeps the grid (the sticky rail's containing block)
               at least a viewport tall, so the rail actually has room to stick
               even when a sub-tab's content is short — otherwise the grid would
               be only as tall as the rail and it would scroll away. -->
          <div class="min-w-0 sm:min-h-[calc(100dvh-9rem)]">
            <!-- Group sub-nav (KunRadioGroup `pill` choice-chips): only the 话题
                 / Galgame groups bundle several sections. Selecting a chip
                 navigates to that section's default route. -->
            <div
              v-if="activeGroup === 'topic' || activeGroup === 'galgame'"
              class="mb-4"
            >
              <KunRadioGroup
                :model-value="activeSegment"
                :options="
                  activeGroup === 'topic'
                    ? userTopicGroupOptions
                    : userGalgameGroupOptions
                "
                variant="pill"
                orientation="horizontal"
                color="primary"
                size="sm"
                @change="goToSegment"
              />
            </div>

            <!-- delay 0: the whole sub-page is replaced (remount + Suspense) on
                 every left-rail / sub-tab switch, so dim immediately for a
                 visible cue rather than the home feed's flicker-guarded delay. -->
            <KunLoadingDim :loading="isNavLoading" :delay="0">
              <NuxtPage :user="data" />
            </KunLoadingDim>
          </div>
        </div>
      </template>

      <KunNull v-else description="未找到该用户" />
    </template>

    <KunNull v-else description="此用户已被封禁" />
  </div>
</template>
