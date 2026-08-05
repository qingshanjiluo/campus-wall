<script setup lang="ts">
import { useIntersectionObserver } from '@vueuse/core'
import { managementRoleLabel, KUN_USER_STATUS_MAP } from '~/constants/user'

// The full-width profile header (a default KunCard): identity (avatar / name /
// role · status chips / 签名 / 私聊) plus the headline metrics lifted out of the
// 关于 panel (the full stats stay there). On scroll, once the header leaves the
// top, a slim sticky bar (small avatar + name + key stats) fades in below the top
// bar so the identity stays visible; the header itself scrolls away normally (the
// mini bar is `fixed`, so it adds no flow height → no scroll jump).
const props = defineProps<{
  user: UserInfo
}>()

const currentUserId = usePersistUserStore().id
const isSelf = computed(() => currentUserId === props.user.id)

// The headline metrics for the header. The complete stats grid lives in 关于;
// these are the at-a-glance figures worth surfacing on every sub-tab.
const metrics = computed(() => [
  { label: '萌萌点', value: props.user.moemoepoint, accent: true },
  { label: '话题', value: props.user.topic },
  { label: 'Galgame', value: props.user.galgame },
  { label: '评分', value: props.user.galgame_rating },
  { label: '被赞', value: props.user.like },
  { label: '被推', value: props.user.upvote }
])

// Collapse when the header has scrolled above the top bar. rootMargin's -80px
// top inset ≈ the fixed top bar height, so the mini bar appears as the header
// clears it. The observer targets the root wrapper (a plain element) rather than
// the KunCard component. Client-only (VueUse no-ops on SSR) → starts expanded.
const bannerRef = ref<HTMLElement | null>(null)
const collapsed = ref(false)
useIntersectionObserver(
  bannerRef,
  ([entry]) => {
    collapsed.value = !entry?.isIntersecting
  },
  { rootMargin: '-80px 0px 0px 0px' }
)
</script>

<template>
  <div ref="bannerRef">
    <KunCard :is-hoverable="false">
      <div class="flex items-start gap-4">
        <KunAvatar
          class-name="cursor-default shrink-0 relative"
          :is-navigation="false"
          size="original-sm"
          :user="{ id: user.id, name: user.name, avatar: user.avatar }"
          :disable-floating="true"
        />

        <div class="min-w-0 flex-1">
          <h1 class="flex flex-wrap items-center gap-2 text-2xl font-bold">
            <span class="truncate">{{ user.name }}</span>
            <KunButton
              v-if="!isSelf"
              variant="flat"
              size="xs"
              color="primary"
              class-name="gap-1"
              :href="`/message/user/${user.id}`"
            >
              <KunIcon name="lucide:message-circle" />
              私聊
            </KunButton>
            <ReportButton
              v-if="!isSelf"
              subject-kind="user"
              :subject-id="user.id"
              :snapshot="user.name"
              :subject-url="`${kungal.domain.main}/user/${user.id}`"
            />
          </h1>

          <div class="mt-2 flex flex-wrap items-center gap-2">
            <KunChip size="sm" color="primary">
              {{ managementRoleLabel(user.roles) }}
            </KunChip>
            <KunChip
              v-if="user.roles.includes('creator')"
              size="sm"
              color="warning"
            >
              创作者
            </KunChip>
            <KunChip size="sm" color="success">
              {{ KUN_USER_STATUS_MAP[user.status] }}
            </KunChip>
          </div>

          <p
            v-if="user.bio"
            class="text-default-600 mt-2 line-clamp-2 text-sm break-words"
          >
            {{ user.bio }}
          </p>

          <div class="text-default-500 mt-2 text-sm">
            注册于 <KunTime :time="user.created" type="date" show-year />
          </div>
        </div>
      </div>

      <!-- Headline metrics -->
      <div class="mt-5 flex flex-wrap gap-x-8 gap-y-3">
        <div v-for="m in metrics" :key="m.label" class="min-w-14">
          <div :class="cn('text-xl font-bold', m.accent && 'text-secondary')">
            {{ m.value }}
          </div>
          <div class="text-default-500 text-xs">{{ m.label }}</div>
        </div>
      </div>
    </KunCard>

    <!-- Compact sticky bar: fades in once the header scrolls above the top bar.
         `fixed` (not in flow) → no scroll jump; sits below the fixed top bar
         (z-20 < top bar z-30) and mirrors the layout's max-w-7xl + rail offset so
         it aligns with the page content. -->
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 -translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <div v-show="collapsed" class="fixed inset-x-0 top-20 z-20">
        <div class="flex justify-center">
          <div
            class="w-full max-w-7xl min-w-0 px-2 desktop-nav:mr-3 desktop-nav:ml-[104px] desktop-nav:px-0"
          >
            <div
              class="bg-content1 border-kun flex items-center gap-3 rounded-xl border px-4 py-2 shadow-kun-sm backdrop-blur-md"
            >
              <KunAvatar
                class-name="cursor-default shrink-0"
                :is-navigation="false"
                size="sm"
                :user="{ id: user.id, name: user.name, avatar: user.avatar }"
                :disable-floating="true"
              />
              <span class="truncate font-semibold">{{ user.name }}</span>
              <KunChip size="sm" color="primary" class-name="hidden sm:inline-flex">
                {{ managementRoleLabel(user.roles) }}
              </KunChip>

              <div class="text-default-600 ml-auto flex items-center gap-4 text-sm">
                <span class="hidden sm:inline">话题 {{ user.topic }}</span>
                <span class="hidden sm:inline">Galgame {{ user.galgame }}</span>
                <span>
                  萌萌点 <b class="text-secondary">{{ user.moemoepoint }}</b>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
