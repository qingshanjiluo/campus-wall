<script setup lang="ts">
import VueApexCharts from 'vue3-apexcharts'
import type { ApexOptions } from 'apexcharts'

const props = defineProps<{
  id: number
  practicalityData: ToolsetRating
}>()

const colorMode = useColorMode()

const categories = computed(() =>
  Array.from({ length: 5 }, (_, i) => String(i + 1))
)

const countsArray = computed(() =>
  categories.value.map((c) => props.practicalityData.counts[Number(c)] ?? 0)
)

const mineIndex = computed(() =>
  props.practicalityData.mine == null
    ? -1
    : categories.value.indexOf(String(props.practicalityData.mine))
)

// Single "评分人数" series — a horizontal bar chart (条形图), no cumulative line.
const series = computed(() => [{ name: '评分人数', data: countsArray.value }])

// Distributed colors so the viewer's own rating bar stands out in gold (the
// same warning hue as the average star), the rest in primary.
const barColors = computed(() =>
  categories.value.map((_, i) =>
    i === mineIndex.value ? 'var(--color-warning)' : 'var(--color-primary)'
  )
)

const options = computed(
  (): ApexOptions => ({
    chart: {
      id: `practicality-chart-${props.id}`,
      type: 'bar',
      height: 260,
      toolbar: { show: false },
      fontFamily: 'inherit'
    },
    plotOptions: {
      bar: {
        horizontal: true,
        distributed: true,
        borderRadius: 4,
        barHeight: '60%'
      }
    },
    dataLabels: {
      enabled: true,
      formatter: (val) => String(val ?? ''),
      style: { fontSize: '12px', colors: ['var(--color-default-900)'] }
    },
    colors: barColors.value,
    xaxis: {
      categories: categories.value.map((c) => `${c} 星`),
      labels: { style: { colors: 'var(--color-default-500)' } }
    },
    yaxis: {
      labels: { style: { colors: 'var(--color-default-900)' } }
    },
    tooltip: {
      theme: colorMode.preference,
      y: { formatter: (y: number) => `${y} 人` }
    },
    grid: { borderColor: 'var(--color-default-200)', strokeDashArray: 4 },
    // Distributed bars would otherwise spawn one legend entry per star.
    legend: { show: false }
  })
)
</script>

<template>
  <div class="contents">
    <ClientOnly>
      <div class="space-y-2">
        <div class="flex items-center gap-3">
          <h3 class="font-semibold">工具实用性分布</h3>
          <span class="text-default-500">
            平均
            <span class="text-warning text-lg font-bold">
              {{ practicalityData.avg.toFixed(1) }}
            </span>
            / 5.0
          </span>
        </div>

        <VueApexCharts
          :key="practicalityData.mine ?? 0"
          type="bar"
          height="260"
          :options="options"
          :series="series"
        />
      </div>
    </ClientOnly>
  </div>
</template>
