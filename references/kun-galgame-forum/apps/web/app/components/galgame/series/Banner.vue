<script setup lang="ts">
const props = defineProps<{
  isNSFW: boolean
  galgames: GalgameSeriesSample[]
}>()

// Prefer the U2 derived banner — `g.banner` is the legacy free-form URL
// and is empty for newly-uploaded (covers-only) galgames, so reading it
// directly leaves the carousel blank for fresh series entries.
// Drop covers-less galgames (empty banner) so the montage fills with up to 5
// real covers instead of blank slots, then cap at 5.
const banners = computed(() =>
  props.galgames
    .map((g) => ({
      url: getEffectiveBanner(g),
      thumbhash: g.effective_banner_thumbhash ?? ''
    }))
    .filter((b) => b.url)
    .slice(0, 5)
)

const hoverTranslations = [
  'group-hover:translate-x-0',
  'group-hover:translate-x-[50%] translate-x-[10%]',
  'group-hover:translate-x-[100%] translate-x-[20%]',
  'group-hover:translate-x-[150%] translate-x-[30%]',
  'group-hover:translate-x-[200%] translate-x-[40%]'
]
</script>

<template>
  <div class="relative mb-4 h-32 w-full">
    <KunChip
      variant="solid"
      class="absolute top-2 left-2 z-100"
      :color="isNSFW ? 'danger' : 'success'"
    >
      {{ isNSFW ? 'NSFW' : 'SFW' }}
    </KunChip>

    <div class="absolute inset-0">
      <div
        v-for="(banner, index) in banners"
        :key="index"
        :class="
          cn(
            'absolute aspect-video h-full rounded-lg transition-transform duration-500 ease-out',
            hoverTranslations[index]
          )
        "
        :style="{ zIndex: banners.length - index }"
      >
        <!--
          Variant separator is picked per URL because legacy nitro-era
          banners (image.kungal.com/galgame/N/banner/banner.webp) use
          `-mini` while image_service URLs (hash-addressed) use `_mini`.
          See getEffectiveBanner helper for the detection rule.
        -->
        <KunImage
          :src="withBannerVariant(banner.url, 'mini')"
          :thumbhash="banner.thumbhash"
          :alt="`Series Banner ${index + 1}`"
          class="h-full w-full rounded-lg object-cover"
          loading="lazy"
          format="webp"
          quality="80"
        />
      </div>
    </div>
  </div>
</template>
