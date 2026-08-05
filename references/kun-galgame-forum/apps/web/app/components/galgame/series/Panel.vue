<script setup lang="ts">
// The 系列 block on a game's detail page. The game record names its series by
// identity only (id + name), so the cards — montage, member count, sample
// titles — are fetched here rather than being carried on every galgame detail
// response for the minority of games that belong to a series.
//
// Lazy: the intro tab is the landing tab, and a series card is supporting
// material. It must not hold up the page it decorates.
const props = defineProps<{
  series: GalgameDetailSeriesRef[]
}>()

const ids = computed(() => props.series.map((s) => s.id).join(','))

const { data } = await useKunFetch<{
  series: GalgameSeriesCard[]
  total: number
}>('/galgame-series/cards', {
  lazy: true,
  method: 'GET',
  query: { ids },
  watch: false
})
</script>

<template>
  <div v-if="data?.series.length" class="space-y-3">
    <KunHeader
      name="Galgame 系列"
      description="Galgame 全系列所有 Galgame 作品。例如美少女万华镜 1, 2, 3, 4, 5, 雪女, 外传 就是一个 Galgame 系列"
      scale="h3"
    />

    <div class="grid grid-cols-1 gap-6">
      <GalgameSeriesCard
        v-for="item in data.series"
        :key="item.id"
        :series="item"
      />
    </div>
  </div>
</template>
