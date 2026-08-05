<script setup lang="ts">
// Unified NSFW toggle — sits directly under the brand in both the
// desktop sidebar and the mobile hamburger (the hamburger reuses
// KunLayoutSidebar with force-expanded=true, so the expanded card form
// is what mobile users always see).
//
// Visibility rule: this affordance is only rendered when NSFW is OFF.
// Once the user has enabled NSFW the card disappears entirely — it
// served its purpose (telling them part of the catalog was hidden) and
// further reminders would be noise. The setting-panel toggle stays
// available as the long-term off-switch path.
//
// Color semantics — per product spec:
//   off (sfw / unset) → danger  — visually highlights that SFW filter is
//                                 hiding part of the catalog.
//
// The KunSwitch is rendered as a visible affordance on the right side of
// the card so users immediately see "this is a toggle". Clicking either
// the whole card or just the switch flips the state. Toggling reloads
// the page so SSR-time fetches (home / list endpoints, sitemap-aware
// paths) re-run under the new content_limit semantics; mirrors the
// existing setting-panel toggle.
//
// The outer container is a `<div role="button">` rather than `<button>`
// because `<button>` cannot contain an interactive descendant per the
// HTML spec (KunSwitch is a `<label>` wrapping a checkbox).

withDefaults(
  defineProps<{
    isCollapsed?: boolean
  }>(),
  { isCollapsed: false }
)

const { showKUNGalgameContentLimit } = storeToRefs(usePersistSettingsStore())

const isEnabled = computed(
  () =>
    showKUNGalgameContentLimit.value === 'nsfw' ||
    showKUNGalgameContentLimit.value === 'all'
)

const toggle = () => {
  showKUNGalgameContentLimit.value = isEnabled.value ? 'sfw' : 'nsfw'
  if (import.meta.client) {
    location.reload()
  }
}
</script>

<template>
  <!--
    Once NSFW is on, the sidebar entry disappears (`v-if="!isEnabled"`).
    Users turn it back off via /user/:id/setting (KunSettingPanelComponentsNSFW).
  -->
  <template v-if="!isEnabled">
    <!-- class-name overrides KunTooltip's default `inline-block` wrapper to a
         full-width block so the inner `w-full` button fills the collapsed
         column and its icon stays centered (inline-block would shrink + left-align). -->
    <KunTooltip
      v-if="isCollapsed"
      text="点击开启 NSFW 模式"
      position="right"
      class-name="block w-full"
    >
      <button
        type="button"
        :class="
          cn(
            'flex w-full items-center justify-center rounded-lg border p-2 transition-colors cursor-pointer',
            'border-danger/40 bg-danger/10 text-danger-700 hover:bg-danger/20 dark:text-danger-300'
          )
        "
        aria-label="NSFW 模式已关闭"
        @click="toggle"
      >
        <KunIcon class="size-5" name="lucide:eye-off" />
      </button>
    </KunTooltip>

    <div
      v-else
      role="button"
      tabindex="0"
      :aria-pressed="false"
      :class="
        cn(
          'w-full rounded-lg border px-3 py-2 text-left transition-colors cursor-pointer outline-none focus-visible:ring-2',
          'border-danger/40 bg-danger/10 text-danger-700 hover:bg-danger/20 focus-visible:ring-danger/40 dark:text-danger-300'
        )
      "
      @click="toggle"
      @keydown.enter.prevent="toggle"
      @keydown.space.prevent="toggle"
    >
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-2 text-sm font-semibold">
          <KunIcon class="size-4 shrink-0" name="lucide:eye-off" />
          <span>NSFW 模式已关闭</span>
        </div>
        <!--
          @click.stop so the switch's own click only triggers `toggle`
          once via @update:model-value — without it the click would also
          bubble to the wrapper div's @click and fire `toggle` a second
          time, immediately reverting the change.
        -->
        <div class="shrink-0" @click.stop>
          <KunSwitch :model-value="false" @update:model-value="toggle" />
        </div>
      </div>
      <p class="mt-1 text-xs opacity-80">
        部分 R18 Galgame 不可见, 点击切换 NSFW 模式
      </p>
    </div>
  </template>
</template>
