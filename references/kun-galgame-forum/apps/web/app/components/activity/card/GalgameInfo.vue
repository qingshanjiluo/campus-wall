<script setup lang="ts">
// Shared galgame "info area" for feed cards: cover (left) + name · 由X制作发售于Y ·
// intro (right). Used by BOTH the new-galgame card and the edit card so an edit
// shows the exact same galgame summary as a creation. Fields come from the
// hydrated GalgameActivityData (developer/intro require the wiki detail brief,
// which the backend fetches for creation + edit alike).
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as GalgameActivityData | undefined
)
const gid = computed(() => data.value?.galgame_id ?? 0)
const detailLink = computed(() =>
  gid.value ? `/galgame/${gid.value}` : props.activity.link
)

// 由 <制作会社> 制作，发售于 <发售日期> — each half shown only if present.
const madeBy = computed(() => {
  const parts: string[] = []
  if (data.value?.developer) parts.push(`由 ${data.value.developer} 制作`)
  if (data.value?.release_date) parts.push(`发售于 ${data.value.release_date}`)
  return parts.join('，')
})
</script>

<template>
  <div class="flex items-start gap-3">
    <KunLink :to="detailLink" class-name="shrink-0">
      <div
        class="bg-default-100 aspect-video w-32 overflow-hidden rounded-lg sm:w-44"
      >
        <img
          v-if="data?.cover_hash"
          :src="imageHashUrl(imageCdnBase(), data.cover_hash, 'mini')"
          :alt="data?.name"
          loading="lazy"
          class="h-full w-full object-cover"
        />
      </div>
    </KunLink>

    <div class="min-w-0 flex-1 space-y-1">
      <KunLink
        underline="none"
        color="default"
        :to="detailLink"
        class-name="hover:text-primary block"
      >
        <h3 class="line-clamp-2 font-medium break-all">
          {{ data?.name || activity.content }}
        </h3>
      </KunLink>
      <p v-if="madeBy" class="text-default-700 text-sm break-all">
        {{ madeBy }}
      </p>
      <p
        v-if="data?.intro"
        class="text-default-500 line-clamp-3 text-sm break-all"
      >
        {{ markdownToText(data.intro) }}
      </p>
    </div>
  </div>
</template>
