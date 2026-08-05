<script setup lang="ts">
import { KUN_FEED_KIND_GROUPS } from '~/constants/activity'

// 主页 feed tabs. The tabs themselves (name / icon / order / existence) are
// FIXED; the only thing editable here is which activity "kinds" each tab shows.
// Single-editor UX: pick a tab above, toggle its kinds below — so the panel
// shows one kind-set at a time instead of stacking every tab's editor (which
// overflowed the modal).
const settings = usePersistSettingsStore()
const { feedTabs } = storeToRefs(settings)

const editingId = ref(feedTabs.value[0]?.id ?? '')
const tabOptions = computed(() =>
  feedTabs.value.map((t) => ({ value: t.id, label: t.name }))
)
const editingTab = computed(() =>
  feedTabs.value.find((t) => t.id === editingId.value)
)

// If the selected tab vanished (e.g. after 恢复默认), fall back to the first.
watchEffect(() => {
  if (
    feedTabs.value.length &&
    !feedTabs.value.some((t) => t.id === editingId.value)
  ) {
    editingId.value = feedTabs.value[0]!.id
  }
})

const toggleKind = (kind: string) => {
  const tab = editingTab.value
  if (!tab) return
  const i = tab.kinds.indexOf(kind)
  if (i >= 0) {
    tab.kinds.splice(i, 1)
  } else {
    tab.kinds.push(kind)
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-start justify-between gap-4">
      <div class="space-y-0.5">
        <p class="text-default-700 font-medium">主页动态标签</p>
        <p class="text-default-500 text-sm">
          选择一个标签，勾选它要展示的动态种类。
        </p>
      </div>
      <KunButton
        size="sm"
        variant="light"
        class="shrink-0"
        @click="settings.resetKUNGalgameFeedTabs()"
      >
        恢复默认
      </KunButton>
    </div>

    <!-- Pick which tab to edit -->
    <KunSelect v-model="editingId" :options="tabOptions" />

    <!-- Kinds for the selected tab -->
    <div v-if="editingTab" class="space-y-3">
      <div
        v-for="group in KUN_FEED_KIND_GROUPS"
        :key="group.label"
        class="space-y-1.5"
      >
        <p class="text-default-400 text-xs">{{ group.label }}</p>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="kind in group.kinds"
            :key="kind.value"
            type="button"
            :class="
              cn(
                'flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs transition-colors',
                editingTab?.kinds.includes(kind.value)
                  ? 'border-primary bg-primary/10 text-primary'
                  : 'border-default-200 text-default-600 hover:bg-default-100'
              )
            "
            @click="toggleKind(kind.value)"
          >
            <KunIcon :name="kind.icon" class="text-sm" />
            {{ kind.label }}
          </button>
        </div>
      </div>

      <p v-if="!editingTab?.kinds.length" class="text-warning text-xs">
        该标签未选择任何种类，将不会显示内容。
      </p>
    </div>
  </div>
</template>
