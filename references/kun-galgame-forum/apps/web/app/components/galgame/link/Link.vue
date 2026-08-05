<script setup lang="ts">
// Read-only display of the galgame's related links. Inline list — no
// card, no header, no per-row controls. Creation / deletion of links
// is part of the "重新编辑 Galgame" PR flow now (pr/Links.vue handles
// the bulk-replace POST `/galgame/:gid/links`); no single-link create
// endpoint exists on the Go side so the previous standalone add/delete
// UI was effectively dead.
const route = useRoute()
const gid = computed(() => parseInt((route.params as { gid: string }).gid))

const { data } = await useKunFetch<GalgameLink[]>(
  `/galgame/${gid.value}/link/all`,
  {
    lazy: true,
    method: 'GET',
    query: { galgame_id: gid.value },
    watch: false
  }
)
</script>

<template>
  <div v-if="data?.length" class="flex flex-wrap gap-x-3 gap-y-1">
    <KunLink
      v-for="(link, index) in data"
      :key="index"
      :to="link.link"
      target="_blank"
      rel="noopener noreferrer"
      size="sm"
      color="default"
      class-name="text-default-500 hover:text-default-700"
    >
      {{ link.name }}
    </KunLink>
  </div>
</template>
