<script setup lang="ts">
// Emitted whenever a menu item is activated so the parent popover (Avatar.vue)
// can dismiss itself — KunPopover stays open on inside-clicks by design.
import type { KnownAccount } from '~/composables/useKnownAccounts'

const emit = defineEmits<{ close: [] }>()

const { id, sub, name, moemoepoint, isCheckIn } = storeToRefs(
  usePersistUserStore()
)
const { canModerate, isCreator } = useRole()
const { accounts } = useKnownAccounts()
const route = useRoute()
const {
  messageStatus,
  showKUNGalgameMoemoepointLog,
  showKUNGalgameLogout,
  showKUNGalgameCreatorApply
} = storeToRefs(useTempSettingStore())

const isShowMessageDot = computed(() => messageStatus.value === 'new')

// "创作者申请" entry — only for plain users without the creator role.
// Moderators / admins (canModerate) already publish galgames directly, and
// existing creators don't need to apply, so both are excluded.
const showCreatorApply = computed(() => !canModerate.value && !isCreator.value)

// ── Account switching (docs/oauth/09-account-switching.md §3.6) ──────────────
// Forum is a BFF, so the menu list is this device's localStorage "known accounts"
// cache and every switch is a top-level authorize redirect (the OP's session bag
// decides the outcome — silent while the account is in the bag, re-login for an
// admin / one logged out elsewhere). The active account already shows at the top
// of the menu, so the switch list is the OTHERS; 添加新账号 is always available.
const showAccountSwitch = ref(false)
const switchableAccounts = computed(() =>
  accounts.value.filter((a) => a.sub !== sub.value)
)

const onSwitchAccount = (account: KnownAccount) => {
  emit('close')
  startOAuthSwitchAccount(account.sub, route.fullPath)
}
const onAddAccount = () => {
  emit('close')
  startOAuthAddAccount(route.fullPath)
}
// Switching INTO an admin / ren account forces an OP re-login (step-up §3.5);
// flag it so the choice isn't surprising. The OP enforces it regardless.
const needsReauth = (account: KnownAccount) =>
  (account.roles ?? []).some((r) => r === 'admin' || r === 'ren')

// The modal is mounted at the app.vue root (same as 萌萌点明细 / 退出登录) so it
// survives this popover unmounting on click-away.
const openCreatorApply = () => {
  emit('close')
  showKUNGalgameCreatorApply.value = true
}

// Opens the 萌萌点明细 modal, which is mounted at the stable app.vue root
// (this menu lives inside a popover that unmounts on click-away).
const openMoemoepointLog = () => {
  emit('close')
  showKUNGalgameMoemoepointLog.value = true
}

const handleCheckIn = async () => {
  emit('close')
  isCheckIn.value = true

  const result = await kunFetch<number>('/user/check-in', {
    method: 'POST'
  })

  if (result === null) {
    return
  }

  moemoepoint.value += result

  if (result === 0) {
    useKunLoliInfo(
      '杂~~~鱼~♡杂鱼~♡ 臭杂鱼♡. 签到成功，您今日什么也没获得...',
      5000
    )
  } else if (result === 7) {
    useKunLoliInfo('杂鱼~♡♡♡♡♡. 签到成功, 您今日好运获得了 7 萌萌点哦!', 5000)
  } else {
    useKunLoliInfo(`杂~~~鱼~♡. 签到成功，您今日获得了 ${result} 萌萌点`, 5000)
  }
}

// Opens the logout scope chooser, mounted at the stable app.vue root. This
// menu lives inside a popover that v-if-unmounts on click-away, so a modal
// rendered HERE would die before showing — the cause of "点退出登录没有反应".
// The actual modal + handlers live in top-bar/Logout.vue, self-bound to the
// temp-store flag (same pattern as the 萌萌点明细 modal).
const openLogout = () => {
  emit('close')
  showKUNGalgameLogout.value = true
}
</script>

<template>
  <div class="flex flex-col gap-1">
    <div class="px-2 py-1">
      <p class="truncate font-semibold">{{ name }}</p>
    </div>

    <!-- 萌萌点 row doubles as the entry to the 明细 modal: "🍭 萌萌点 …… 8888 >"
         guides the user to click to view the full ledger. -->
    <button
      type="button"
      class="hover:bg-default-100 flex w-full items-center justify-between rounded-lg px-2 py-2 text-sm transition-colors"
      @click="openMoemoepointLog"
    >
      <span class="flex items-center gap-2">
        <KunIcon class="text-secondary size-4" name="lucide:lollipop" />
        萌萌点
      </span>
      <span class="flex items-center gap-1">
        <span class="text-secondary font-bold tabular-nums">
          {{ moemoepoint }}
        </span>
        <KunIcon
          class="text-foreground/40 size-4"
          name="lucide:chevron-right"
        />
      </span>
    </button>

    <KunButton
      v-if="!isCheckIn"
      variant="light"
      color="secondary"
      size="sm"
      :full-width="true"
      rounded="md"
      class-name="justify-between"
      @click="handleCheckIn"
    >
      <span class="flex items-center gap-2">
        <KunIcon class="size-4" name="lucide:calendar-check" />
        每日签到
      </span>
      <KunIcon class="text-secondary-500 size-5" name="lucide:sparkles" />
    </KunButton>

    <NuxtLink
      :to="`/user/${id}`"
      class="hover:bg-default-100 flex items-center gap-2 rounded-lg px-2 py-2 text-sm transition-colors"
      @click="emit('close')"
    >
      <KunIcon class="size-4" name="lucide:user-round" />
      个人主页
    </NuxtLink>

    <NuxtLink
      to="/message"
      class="hover:bg-default-100 flex items-center gap-2 rounded-lg px-2 py-2 text-sm transition-colors"
      @click="emit('close')"
    >
      <KunIcon class="size-4" name="lucide:mail" />
      我的消息
      <span
        v-if="isShowMessageDot"
        class="bg-secondary-500 ml-auto size-2 rounded-full"
      />
    </NuxtLink>

    <!-- 管理系统 — moderators / admins (canModerate). The /admin route is
         server-gated too; this just hides the entry from plain users, matching
         moyu's isModerator check. -->
    <NuxtLink
      v-if="canModerate"
      to="/admin/overview"
      class="hover:bg-default-100 flex items-center gap-2 rounded-lg px-2 py-2 text-sm transition-colors"
      @click="emit('close')"
    >
      <KunIcon class="size-4" name="lucide:shield-check" />
      管理系统
    </NuxtLink>

    <!-- 创作者申请 — accent-styled so it stands out as an aspirational action. -->
    <button
      v-if="showCreatorApply"
      type="button"
      class="text-primary hover:bg-primary-50 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-sm font-medium transition-colors"
      @click="openCreatorApply"
    >
      <KunIcon class="size-4" name="lucide:sparkles" />
      创作者申请
      <KunIcon
        class="text-primary/50 ml-auto size-4"
        name="lucide:chevron-right"
      />
    </button>

    <!-- 账号切换 — this device's other accounts + 添加新账号. Each switch is a
         top-level authorize redirect (forum is a BFF; the OP bag is the source of
         truth). docs/oauth/09-account-switching.md §3.6. -->
    <button
      type="button"
      class="hover:bg-default-100 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-sm transition-colors"
      @click="showAccountSwitch = !showAccountSwitch"
    >
      <KunIcon class="size-4" name="lucide:users-round" />
      账号切换
      <KunIcon
        class="text-foreground/40 ml-auto size-4 transition-transform"
        :class="showAccountSwitch ? 'rotate-90' : ''"
        name="lucide:chevron-right"
      />
    </button>

    <div v-if="showAccountSwitch" class="flex flex-col gap-1 pl-2">
      <button
        v-for="account in switchableAccounts"
        :key="account.sub"
        type="button"
        class="hover:bg-default-100 flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm transition-colors"
        @click="onSwitchAccount(account)"
      >
        <KunAvatar
          :user="{ id: account.id, name: account.name, avatar: account.avatar }"
          size="sm"
          :is-navigation="false"
          :disable-floating="true"
        />
        <span class="flex min-w-0 flex-col items-start">
          <span class="max-w-40 truncate">{{ account.name }}</span>
          <span v-if="needsReauth(account)" class="text-default-400 text-xs">
            切换需重新登录
          </span>
        </span>
      </button>

      <button
        type="button"
        class="text-primary hover:bg-primary-50 flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-sm font-medium transition-colors"
        @click="onAddAccount"
      >
        <KunIcon class="size-4" name="lucide:plus" />
        添加新账号
      </button>
    </div>

    <KunButton
      variant="light"
      color="danger"
      size="sm"
      :full-width="true"
      rounded="md"
      class-name="justify-start"
      @click="openLogout"
    >
      <KunIcon class="size-4" name="lucide:log-out" />
      退出登录
    </KunButton>
  </div>
</template>
