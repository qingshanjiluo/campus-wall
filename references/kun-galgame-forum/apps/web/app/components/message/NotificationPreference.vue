<script setup lang="ts">
// Notification-category preferences (BE migration 053), reused both in the
// account settings page and in a modal on /message/notice. Each switch is
// "接收该类通知" — ON = receive, OFF = mute. Muting only stops the category from
// driving the top-bar red dot / unread badges; the messages are still kept and
// stay visible in the notification center.
//
// Container-agnostic: renders just the heading + tabs + switches, no outer
// KunCard/modal frame — the host provides that. Catalog keys mirror the server
// whitelist: message.type values, the "chat" pseudo key, and namespaced
// "wiki:*" keys. Official system broadcasts are intentionally non-mutable, so
// they have no switch here.
import { notificationCategoryGroups } from '~/constants/notification'

const tabs = notificationCategoryGroups

const tabItems = tabs.map(({ value, textValue, icon }) => ({
  value,
  textValue,
  icon
}))
const activeTab = ref('interaction')

// Items of the currently-selected tab (mirrors Resource.vue's activeBucket):
// render a single list rather than v-show-ing every panel.
const activeItems = computed(
  () => tabs.find((t) => t.value === activeTab.value)?.items ?? []
)

const allKeys = tabs.flatMap((t) => t.items.map((i) => i.key))

// enabled[key] === true → receiving (not muted). Defaults to true.
const enabled = reactive<Record<string, boolean>>(
  Object.fromEntries(allKeys.map((k) => [k, true]))
)
const isLoading = ref(true)

const applyMuted = (muted: string[]) => {
  const mutedSet = new Set(muted)
  for (const key of allKeys) {
    enabled[key] = !mutedSet.has(key)
  }
}

const collectMuted = () => allKeys.filter((key) => !enabled[key])

// Fetch the authoritative muted set and apply it locally.
const load = async () => {
  const result = await kunFetch<NotificationPreference>(
    '/user/notification-preferences'
  )
  if (result?.muted_types) {
    applyMuted(result.muted_types)
  }
}

// Persist the current desired state. One request is kept in flight and rapid
// toggles are coalesced (queued) so the last state always wins — concurrent
// PUTs can't overwrite each other out of order. Each PUT sends the full set, so
// on failure we re-sync from the server rather than track per-toggle rollbacks.
const isSaving = ref(false)
let queued = false

const persist = async () => {
  if (isSaving.value) {
    queued = true
    return
  }
  isSaving.value = true
  try {
    do {
      queued = false
      const result = await kunFetch<NotificationPreference>(
        '/user/notification-preferences',
        { method: 'PUT', body: { muted_types: collectMuted() } }
      )
      if (!result) {
        throw new Error('save failed')
      }
    } while (queued)
  } catch {
    useMessage('保存失败，请稍后重试', 'error')
    await load()
  } finally {
    isSaving.value = false
  }
}

const onToggle = (key: string, value: boolean) => {
  enabled[key] = value
  void persist()
}

onMounted(async () => {
  await load()
  isLoading.value = false
})
</script>

<template>
  <div class="space-y-4">
    <div>
      <span class="text-xl">消息通知</span>
      <p class="text-default-500 text-sm">
        关闭某类通知后，它不会再让顶栏红点闪烁或点亮未读角标，但消息仍会保留在通知中心里，你随时可以进去查看。
      </p>
    </div>

    <KunTab
      v-model="activeTab"
      :items="tabItems"
      variant="underlined"
      color="primary"
      size="sm"
    />

    <div class="divide-default-100 divide-y">
      <div
        v-for="item in activeItems"
        :key="item.key"
        class="flex items-center justify-between py-2"
      >
        <span class="text-default-700 text-sm">{{ item.label }}</span>
        <KunSwitch
          :model-value="enabled[item.key] ?? true"
          :disabled="isLoading"
          @update:model-value="(v: boolean) => onToggle(item.key, v)"
        />
      </div>
    </div>
  </div>
</template>
