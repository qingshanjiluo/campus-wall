<script setup lang="ts">
import { useContentBlurUp, useContentLightbox } from '@kungal/ui-vue'

// Renders a topic's covers. Each entry is a /image/<hash> content token resolved
// to an absolute CDN URL (imageTokenUrl: skips @nuxt/image IPX + the /image 302
// hop). `meta` (keyed by the same token) carries each cover's intrinsic dims +
// ThumbHash.
//
// Multi-cover layout is a SINGLE uniform-height row: every cover shares one
// height and keeps its OWN aspect ratio (width = height × ratio), so nothing is
// ever cropped and there are no letterbox bars — a tall cover (text screenshot)
// is just a narrow full-height column next to wider ones. The row never wraps;
// it scrolls horizontally when it overflows (KunScrollShadow fades the edges to
// hint there's more). Width is reserved up-front from the real dims when known
// (no reflow); otherwise the browser sizes it from the natural image on load.
const props = defineProps<{
  images: string[]
  meta?: Record<string, KunImageMeta>
  // Opt in to the fullscreen viewer (detail pages). Off in feeds — see below.
  zoomable?: boolean
}>()

const shown = computed(() => props.images.slice(0, 9))
const isSingle = computed(() => shown.value.length === 1)

const metaOf = (token: string): KunImageMeta | undefined => props.meta?.[token]

// CSS aspect-ratio from real dims so the slot reserves its width before the image
// loads (no layout shift); undefined → natural width resolves on load.
const aspectOf = (token: string): string | undefined => {
  const m = metaOf(token)
  return m?.width && m?.height ? `${m.width} / ${m.height}` : undefined
}

// Blur-up via plain <img data-thumbhash> + the kun-ui composable. (KunImage can't
// size itself to the image's natural width, which is exactly what keeps every
// cover uncropped here — so we drive the placeholder ourselves.)
const root = ref<HTMLElement | null>(null)
useContentBlurUp(root)

// Fullscreen viewer, OPT-IN. Body images get one for free inside KunContent, but
// these covers are bare <img> outside it, so on a detail page clicking one did
// nothing — and the multi-cover row caps every cover at h-40, exactly the case
// where you most want a closer look.
//
// It must stay opt-in: in the three feed/activity cards the grid sits inside a
// KunLink to the topic, and the composable's handler preventDefaults the click.
// Attaching it there would swallow the navigation the cover exists to trigger —
// a cover in a list is the thumbnail that earns the click, not a picture to zoom.
//
// Passing a permanently-null ref is what disables it: the composable reads the
// ref once in its own onMounted, so a ref that never receives the element simply
// never gets a listener. Reading the prop at setup is fine — every call site
// passes a literal, so it never flips at runtime.
const lightboxRoot = props.zoomable ? root : ref<HTMLElement | null>(null)

// The composable collects every <img> inside its container AT CLICK TIME, so the
// lightbox renders OUTSIDE that container: nested, its own full-size image and
// thumbnail strip would join the gallery on the next click. Same reason
// pm/Item.vue keeps the two siblings.
const { isLightboxOpen, images, currentImageIndex } =
  useContentLightbox(lightboxRoot)
</script>

<template>
  <div v-if="shown.length">
    <div ref="root">
      <!-- Single: the element is sized to the image itself (capped at the card
         width / 24rem height, ratio preserved) — NOT w-full + object-contain,
         which would letterbox a portrait into the left of a full-width box and
         leave the rounded right corners stranded in the empty area (square-right
         bug). Here the box equals the image, so all four corners round cleanly. -->
      <!-- width/height ATTRS (not just an aspect-ratio style) so the box reserves
         BEFORE load: an img with only `aspect-ratio` + `max-w-full` and no width
         has auto (=0) width until the image loads, so nothing was reserved (no
         placeholder) and the thumbhash background had 0 area to show in. The attrs
         give the browser the intrinsic size; max-w/max-h + the ratio still cap it. -->
      <img
        v-if="isSingle"
        :src="imageTokenUrl(shown[0]!)"
        :data-thumbhash="metaOf(shown[0]!)?.thumbhash || undefined"
        :width="metaOf(shown[0]!)?.width || undefined"
        :height="metaOf(shown[0]!)?.height || undefined"
        :style="
          aspectOf(shown[0]!) ? { aspectRatio: aspectOf(shown[0]!) } : undefined
        "
        alt="话题封面"
        loading="lazy"
        :class="
          cn(
            'block h-auto max-h-96 w-auto max-w-full rounded-lg',
            zoomable && 'cursor-zoom-in'
          )
        "
      />

      <!-- Multi: one uniform-height row, each cover at its own aspect ratio (no crop,
         no bars); overflow scrolls horizontally instead of wrapping. -->
      <KunScrollShadow
        v-else
        axis="horizontal"
        shadow-size="2rem"
        scrollbar="thin"
      >
        <div class="flex gap-1.5">
          <img
            v-for="(token, idx) in shown"
            :key="`${idx}-${token}`"
            :src="imageTokenUrl(token)"
            :data-thumbhash="metaOf(token)?.thumbhash || undefined"
            :style="
              aspectOf(token) ? { aspectRatio: aspectOf(token) } : undefined
            "
            alt="话题封面"
            loading="lazy"
            :class="
              cn(
                'h-40 w-auto shrink-0 rounded-lg object-contain',
                zoomable && 'cursor-zoom-in'
              )
            "
          />
        </div>
      </KunScrollShadow>
    </div>

    <KunLightbox
      v-if="zoomable"
      v-model:is-open="isLightboxOpen"
      :images="images"
      :initial-index="currentImageIndex"
    />
  </div>
</template>
