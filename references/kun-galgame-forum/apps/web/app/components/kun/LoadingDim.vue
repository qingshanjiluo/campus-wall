<script setup lang="ts">
// Stale-while-revalidate dim for content that reloads on a tab / nav switch.
//
// The problem: a tab (home feed, search, profile left-rail) highlights the
// instant it's clicked, but the new data loads async — so the OLD content keeps
// sitting there with no cue that it's stale. While `loading`, this dims the
// slotted content to 0.5 opacity, makes it `inert` (no clicks on about-to-be-
// replaced content) and sets `aria-busy`.
//
// It uses a DELAYED fade (the same useDeferredValue trick as KunTabPanel 2.10):
// the dim only starts after 0.2s, so a load that resolves quickly clears
// `loading` before it ever paints — fast switches never flicker; only a
// genuinely slow load (the case that actually needs a cue) visibly dims. The
// snap-back to full opacity is instant.
const props = withDefaults(
  defineProps<{
    loading?: boolean
    // Delay (ms) before the dim starts fading in — the flicker guard. A load
    // that resolves within it never dims. Default 200 suits stale-while-
    // revalidate (home feed). Use 0 for nav switches where the whole content is
    // replaced (profile), so the cue is actually visible on quick loads.
    delay?: number
  }>(),
  { delay: 200 }
)
</script>

<template>
  <div
    :class="['kun-loading-dim', { 'kun-loading-dim--busy': loading }]"
    :style="{ '--kun-dim-delay': `${props.delay}ms` }"
    :inert="loading || undefined"
    :aria-busy="loading || undefined"
  >
    <slot />
  </div>
</template>

<style scoped>
.kun-loading-dim {
  opacity: 1;
}
/* The transition exists ONLY while busy, so removing the class reverts to full
   opacity with no transition (instant snap-back). Adding it fades to 0.5 after
   `delay` → a load shorter than the delay never dims. */
.kun-loading-dim--busy {
  opacity: 0.5;
  transition: opacity 0.2s var(--kun-dim-delay, 200ms) linear;
}
@media (prefers-reduced-motion: reduce) {
  .kun-loading-dim--busy {
    transition: none;
  }
}
</style>
