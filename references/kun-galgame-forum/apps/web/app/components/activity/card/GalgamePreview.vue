<script setup lang="ts">
// Shared "embedded galgame preview" frame for feed cards: a bordered, clickable
// row with the galgame cover (16:9, matching the canonical galgame card) on the
// LEFT and type-specific info in the default slot on the RIGHT. Used by the
// new-galgame card and the galgame-reference card (edit / PR / comment / rating /
// resource). The cover loads the lightweight `mini` variant from the CDN.
defineProps<{
  to: string
  coverHash?: string
  name?: string
}>()
</script>

<template>
  <KunLink
    underline="none"
    color="default"
    :to="to"
    class-name="group border-default-200 hover:border-primary/50 hover:bg-default-100/40 flex gap-3 rounded-xl border p-2.5 transition-colors"
  >
    <div
      class="bg-default-100 aspect-video w-32 shrink-0 overflow-hidden rounded-lg sm:w-40"
    >
      <img
        v-if="coverHash"
        :src="imageHashUrl(imageCdnBase(), coverHash, 'mini')"
        :alt="name"
        loading="lazy"
        class="h-full w-full object-cover"
      />
    </div>
    <div class="flex min-w-0 flex-1 flex-col justify-center gap-1.5">
      <slot />
    </div>
  </KunLink>
</template>
