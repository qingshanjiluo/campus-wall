<script setup lang="ts">
// U2 screenshot gallery — read-only display on the detail page.
//
// Clicking a thumbnail opens a connected KunLightboxGallery (prev/next + zoom
// across the whole gallery in one overlay), the same lightbox the banner uses.
//
// Per-rating SFW gate — two independent axes, both filtered client-side so the
// editor's replace-all submit never loses rows (see GalleryFilter.vue):
//   色情 (sexual)   — global NSFW mode shows every level; SFW shows only the
//                     levels the viewer opted into.
//   暴力 (violence) — independent per-level opt-in (default off, behind a
//                     warning); the NSFW mode does NOT unlock it.
// An image shows iff BOTH its sexual and violence levels are permitted. The
// filter control stays reachable even when everything is hidden, so a fully
// rated gallery can still be revealed. Selections persist and the NSFW toggle
// reloads, so SSR and client agree.
//
// Image source: galgameImageSrc resolves each row's cdn_url (kungal injects it
// via rewriteBanners) and falls back to the /image/<hash> token (web 302
// middleware) when a row lacks cdn_url — so a screenshot still shows from
// image_hash alone instead of being silently dropped.
const props = defineProps<{
  screenshots: GalgameScreenshot[]
}>()

const {
  showKUNGalgameContentLimit,
  showKUNGalgameGallerySexualLevels: sexualLevels,
  showKUNGalgameGalleryViolenceLevels: violenceLevels
} = storeToRefs(usePersistSettingsStore())

const showNsfw = computed(
  () =>
    showKUNGalgameContentLimit.value === 'nsfw' ||
    showKUNGalgameContentLimit.value === 'all'
)

// 色情: NSFW reveals every level; otherwise unrated (0) + opted-in levels.
// 暴力: unrated (0) + opted-in levels only, independent of the NSFW mode.
const sexualOk = (s: GalgameScreenshot) =>
  showNsfw.value || s.sexual === 0 || sexualLevels.value.includes(s.sexual)
const violenceOk = (s: GalgameScreenshot) =>
  s.violence === 0 || violenceLevels.value.includes(s.violence)

const allShots = computed(() =>
  [...(props.screenshots ?? [])].filter((s) => !!s.image_hash)
)

const sorted = computed(() =>
  allShots.value
    .filter((s) => sexualOk(s) && violenceOk(s))
    .sort((a, b) => {
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
      return a.image_hash.localeCompare(b.image_hash)
    })
)

const hiddenCount = computed(() => allShots.value.length - sorted.value.length)

// Only surface the filter when there's something to gate — a gallery of purely
// unrated screenshots needs no control.
const hasRated = computed(() =>
  allShots.value.some((s) => s.sexual >= 1 || s.violence >= 1)
)

// Per-level image counts (level 1/2/3 → n), so the filter can show how many
// images each toggle reveals/hides.
const countLevels = (axis: 'sexual' | 'violence'): Record<number, number> => {
  const counts: Record<number, number> = { 1: 0, 2: 0, 3: 0 }
  for (const s of allShots.value) {
    const level = s[axis]
    if (level >= 1 && level <= 3) counts[level] = (counts[level] ?? 0) + 1
  }
  return counts
}
const sexualCounts = computed(() => countLevels('sexual'))
const violenceCounts = computed(() => countLevels('violence'))

// Grid thumbnails load the `mini` variant (every screenshot has one in
// image_service — verified) instead of the full 1920×1080 image; the lightbox
// still opens the full-resolution src. withImageVariant only rewrites real
// image_service .webp URLs, so the /image/<hash> fallback (a row without
// cdn_url) returns the full image unchanged — acceptable.
const thumbSrc = (s: GalgameScreenshot) =>
  s.cdn_url ? withImageVariant(s.cdn_url, 'mini') : galgameImageSrc(s)

// Per-tile rating rings: outer band = 色情 (warning), inner band = 暴力
// (danger) — same colour mapping as the editor's rating badges; colour depth =
// level (轻/中/高). Rendered as nested INSET box-shadows on a pointer-events-none
// overlay above the image, so they can't be clipped or block clicks. An axis
// with no rating draws nothing; a single rated axis draws one edge ring (its
// colour says which).
const RING_W = 2.5 // px per band
const RING_DEPTH: Record<number, number> = { 1: 60, 2: 80, 3: 100 }
const ringColor = (token: 'warning' | 'danger', level: number) =>
  `color-mix(in oklab, var(--color-${token}) ${RING_DEPTH[level] ?? 100}%, transparent)`

const ratingRing = (s: GalgameScreenshot) => {
  const shadows: string[] = []
  if (s.sexual >= 1) {
    shadows.push(`inset 0 0 0 ${RING_W}px ${ringColor('warning', s.sexual)}`)
  }
  if (s.violence >= 1) {
    const inset = s.sexual >= 1 ? RING_W * 2 : RING_W
    shadows.push(`inset 0 0 0 ${inset}px ${ringColor('danger', s.violence)}`)
  }
  return { boxShadow: shadows.join(', ') }
}
</script>

<template>
  <div v-if="allShots.length" class="space-y-3">
    <div class="flex flex-wrap items-end justify-between gap-2">
      <KunHeader
        name="画廊"
        description="该 Galgame 的截图 / CG 集"
        scale="h3"
      />
      <GalgameGalleryFilter
        v-if="hasRated"
        :show-nsfw="showNsfw"
        :hidden-count="hiddenCount"
        :sexual-counts="sexualCounts"
        :violence-counts="violenceCounts"
      />
    </div>

    <KunLightboxGallery v-if="sorted.length">
      <!-- auto-fill min-width grid: tiles never shrink below ~180px (a fixed
           5-col grid made them tiny in a narrow content column). 2 cols on
           phones, then as many ≥180px columns as fit. -->
      <div
        class="grid grid-cols-2 gap-2 sm:grid-cols-[repeat(auto-fill,minmax(180px,1fr))]"
      >
        <KunLightboxGalleryItem
          v-for="s in sorted"
          :key="s.image_hash"
          :src="galgameImageSrc(s)"
          :alt="s.caption || ''"
          :wrap="false"
          v-slot="{ open }"
        >
          <button
            type="button"
            class="group hover:ring-primary focus:ring-primary relative block w-full overflow-hidden rounded-lg ring-1 ring-transparent transition-all focus:outline-none"
            :aria-label="s.caption || '查看截图'"
            @click="open"
          >
            <KunImage
              :src="thumbSrc(s)"
              :alt="s.caption || ''"
              loading="lazy"
              object-fit="cover"
              :thumbhash="s.thumbhash"
              class="h-full w-full cursor-zoom-in object-cover transition-transform duration-200 group-hover:scale-105"
              :style="{ aspectRatio: '16/9' }"
            />
            <div
              v-if="s.caption"
              class="absolute right-0 bottom-0 left-0 truncate bg-black/50 px-2 py-1 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
            >
              {{ s.caption }}
            </div>
            <!-- rating rings: outer=色情 inner=暴力, depth=level -->
            <div
              v-if="s.sexual >= 1 || s.violence >= 1"
              class="pointer-events-none absolute inset-0 rounded-lg"
              :style="ratingRing(s)"
            />
          </button>
        </KunLightboxGalleryItem>
      </div>
    </KunLightboxGallery>

    <KunNull
      v-else
      :description="`${hiddenCount} 张图片已按分级隐藏，点击「分级筛选」调整`"
    />
  </div>
</template>
