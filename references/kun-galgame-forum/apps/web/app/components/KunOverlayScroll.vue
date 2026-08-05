<script setup lang="ts">
// Project-wide overlay scrollbar: a thin, themed bar (overlayscrollbars, with the
// handle theme picked per color mode below) that replaces the glaring native
// scrollbar. Drop it in wherever a scroll box would otherwise show
// `scrollbar-hide` (no bar) or the raw native bar — pass the height / max-height
// as a class, the slot is the scrollable content.
//
// overlayscrollbars restructures into .os-host > .os-padding > .os-viewport >
// .os-content > <slot>, so the .os-viewport is the REAL scroller, NOT this host.
// Callers that drive scroll imperatively (scrollTo / scrollTop / scrollHeight)
// must go through getViewport() — see message/pm/Container.vue's chat history.
import {
  OverlayScrollbarsComponent,
  type OverlayScrollbarsComponentRef
} from 'overlayscrollbars-vue'
import type { PartialOptions } from 'overlayscrollbars'

const props = withDefaults(
  defineProps<{
    // Defer init to browser-idle (perf). Turn OFF when the consumer needs the
    // viewport synchronously right after mount (e.g. an initial scroll-to-bottom):
    // osInstance() — and thus getViewport() — is null until initialization runs.
    defer?: boolean
  }>(),
  { defer: true }
)

// overlayscrollbars' handle visuals live inside its theme classes, so one IS
// required (theme:null leaves a 0-width, transparent handle). The built-in
// themes are fixed, though: `os-theme-dark` is a dark handle (for light
// surfaces) and `os-theme-light` a light one (for dark surfaces). Pick per
// color-mode so the handle always contrasts with the background — otherwise the
// dark handle stays dark in dark mode and is nearly invisible. Size/rounding use
// the theme's defaults (a thin ~6px rounded handle), which suit us as-is.
const colorMode = useColorMode()
const options = computed<PartialOptions>(() => ({
  scrollbars: {
    theme: colorMode.value === 'dark' ? 'os-theme-light' : 'os-theme-dark',
    // 'leave': show the bar while the pointer is over the area, auto-hide ~500ms
    // after it leaves — an unobtrusive overlay bar, not a permanent fixture.
    autoHide: 'leave',
    autoHideDelay: 500
  }
}))

const osRef = ref<OverlayScrollbarsComponentRef | null>(null)

const getViewport = (): HTMLElement | null =>
  osRef.value?.osInstance()?.elements().viewport ?? null

defineExpose({ getViewport })
</script>

<template>
  <OverlayScrollbarsComponent ref="osRef" :options="options" :defer="props.defer">
    <slot />
  </OverlayScrollbarsComponent>
</template>
