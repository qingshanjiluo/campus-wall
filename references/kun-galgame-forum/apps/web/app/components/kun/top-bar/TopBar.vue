<script setup lang="ts">
import { useWindowScroll } from '@vueuse/core'

withDefaults(
  defineProps<{
    className?: string
  }>(),
  { className: '' }
)

// Match the page content's bounds exactly so the bar lines up with it: content is
// desktop-nav:ml-[104px] (icon rail 80px + ~24px gap) and desktop-nav:mr-3 (12px),
// i.e. spans [104px, 100%-12px]. left-[104px] + w-[calc(100%-116px)] reproduces
// that; the inner mx-auto max-w-7xl then centers identically. Must use the SAME
// desktop-nav gate as the rail (default.vue) — at tablet/touch widths the rail is
// hidden, so the bar must be full-width, not offset by a phantom 104px.
const offsetClass = 'desktop-nav:left-[104px] desktop-nav:w-[calc(100%-116px)]'

// Flat + edge-to-edge + transparent at the very top; once the page scrolls it
// eases to the inset surface (bg-content1 + border + shadow + px-3). Blur is OFF
// by default and turns ON only when scrolled.
//
// Jank guard: the transition list intentionally EXCLUDES backdrop-filter —
// animating it stutters (the reason the bar used to keep blur constant). Here
// the blur SNAPS on at the threshold (one cheap GPU repaint, not a 200ms filter
// animation) while bg / border / shadow / padding fade smoothly. transform-gpu
// keeps the bar on its own layer.
const { y } = useWindowScroll()
const scrolled = computed(() => y.value > 8)
</script>

<template>
  <div
    :class="
      cn(
        'fixed top-0 z-30 mb-3 ml-0 shrink-0',
        'left-0 w-full',
        offsetClass,
        className
      )
    "
  >
    <div
      :class="
        cn(
          'mx-auto flex h-16 w-full max-w-7xl transform-gpu items-center justify-between rounded-b-lg border transition-[background-color,border-color,box-shadow,padding] duration-200',
          scrolled
            ? 'bg-content1 border-kun px-3 shadow-kun-sm backdrop-blur-md'
            : 'border-transparent bg-transparent px-3 shadow-none desktop-nav:px-0'
        )
      "
    >
      <KunTopBarNav />
      <KunTopBarAvatar />
    </div>
  </div>
</template>
