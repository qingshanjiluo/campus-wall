<script setup lang="ts">
const props = defineProps<{
  data: GalgameCalendarMonth
}>()

// Month paging is owned by the parent (it holds the URL-backed `month` ref).
const emit = defineEmits<{
  prev: []
  next: []
  today: []
}>()

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六']

const year = computed(() => Number(props.data.month.slice(0, 4)))
const monthNum = computed(() => Number(props.data.month.slice(5, 7)))

const monthLabel = computed(() => `${year.value} 年 ${monthNum.value} 月`)
const isCurrentMonth = computed(
  () => props.data.month === props.data.today.slice(0, 7)
)
// Backward stops at the data floor; forward is unbounded (parent computes it).
const canGoPrev = computed(() => props.data.month > props.data.meta.min_month)

// Today's day-of-month, or -1 when today isn't in the viewed month.
const todayDay = computed(() =>
  props.data.today.slice(0, 7) === props.data.month
    ? Number(props.data.today.slice(8, 10))
    : -1
)

const dayGames = computed(() => {
  const map = new Map<number, GalgameCard[]>()
  const bucket: GalgameCard[] = []
  for (const game of props.data.items) {
    if (game.release_precision === 'month' || !game.release_date) {
      bucket.push(game)
      continue
    }
    const day = Number(game.release_date.slice(8, 10))
    if (!map.has(day)) {
      map.set(day, [])
    }
    map.get(day)!.push(game)
  }
  return { map, bucket }
})

const countOf = (day: number | null) =>
  day === null ? 0 : (dayGames.value.map.get(day)?.length ?? 0)

const cells = computed<(number | null)[]>(() => {
  const firstWeekday = new Date(year.value, monthNum.value - 1, 1).getDay()
  const daysInMonth = new Date(year.value, monthNum.value, 0).getDate()
  const out: (number | null)[] = []
  for (let i = 0; i < firstWeekday; i++) {
    out.push(null)
  }
  for (let d = 1; d <= daysInMonth; d++) {
    out.push(d)
  }
  while (out.length % 7 !== 0) {
    out.push(null)
  }
  return out
})

// `selected` highlights the active cell; clicking scrolls the (normal page-flow)
// month list to that day's row.
const selected = ref<number | 'bucket' | null>(null)
const defaultSelected = computed<number | 'bucket' | null>(() =>
  todayDay.value > 0 && dayGames.value.map.has(todayDay.value)
    ? todayDay.value
    : null
)
watch(() => props.data.month, () => (selected.value = defaultSelected.value), {
  immediate: true
})

const scrollToRow = (id: string) => {
  if (import.meta.client) {
    document
      .getElementById(`cal-day-${id}`)
      ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}
const pickDay = (day: number | null) => {
  if (day === null || !countOf(day)) {
    return
  }
  selected.value = day
  scrollToRow(`${props.data.month}-${String(day).padStart(2, '0')}`)
}
const pickBucket = () => {
  if (!dayGames.value.bucket.length) {
    return
  }
  selected.value = 'bucket'
  scrollToRow(`${props.data.month}-bucket`)
}
</script>

<template>
  <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-5">
    <!-- LEFT: focus-month calendar. Sticky on wide so it stays put while the
         month list scrolls; full width on mobile. -->
    <div
      class="flex flex-col gap-3 lg:sticky lg:top-20 lg:w-1/3 lg:shrink-0 lg:self-start"
    >
      <div
        class="border-default-200 flex items-center justify-between gap-2 rounded-xl border px-2 py-1.5"
      >
        <KunButton
          variant="light"
          :is-icon-only="true"
          :disabled="!canGoPrev"
          @click="emit('prev')"
        >
          <KunIcon name="lucide:chevron-left" class="size-5" />
        </KunButton>
        <div class="flex flex-col items-center">
          <span class="font-bold">{{ monthLabel }}</span>
          <span class="text-default-400 text-xs">
            共 {{ data.meta.count }} 部
          </span>
        </div>
        <KunButton variant="light" :is-icon-only="true" @click="emit('next')">
          <KunIcon name="lucide:chevron-right" class="size-5" />
        </KunButton>
      </div>

      <div v-if="!isCurrentMonth" class="flex justify-center">
        <KunButton variant="light" size="sm" @click="emit('today')">
          <KunIcon name="lucide:undo-2" class="size-4" />
          回到本月
        </KunButton>
      </div>

      <div class="grid grid-cols-7 gap-1.5">
        <div
          v-for="w in WEEKDAYS"
          :key="w"
          class="text-default-400 py-1 text-center text-xs"
        >
          {{ w }}
        </div>
      </div>

      <div class="grid grid-cols-7 gap-1.5">
        <template v-for="(cell, i) in cells" :key="i">
          <div v-if="cell === null" />
          <!-- Release count as a corner badge (KunBadge). Today is outlined +
               bold, selected gets a soft fill, empty days are muted. -->
          <KunBadge
            v-else
            :count="countOf(cell)"
            :show="countOf(cell) > 0"
            :max="99"
            color="primary"
            size="sm"
            placement="top-right"
          >
            <button
              type="button"
              :disabled="!countOf(cell)"
              class="flex h-12 w-full items-center justify-center rounded-lg border text-sm transition-colors sm:h-14 sm:text-base"
              :class="
                cell === todayDay
                  ? 'border-primary text-primary border-2 font-bold'
                  : selected === cell
                    ? 'border-primary bg-default-100 text-default-600'
                    : countOf(cell)
                      ? 'border-default-200 hover:border-primary text-default-600 cursor-pointer'
                      : 'text-default-400 border-transparent'
              "
              @click="pickDay(cell)"
            >
              {{ cell }}
            </button>
          </KunBadge>
        </template>
      </div>

      <button
        v-if="dayGames.bucket.length"
        type="button"
        class="border-default-200 hover:border-primary flex items-center gap-2 self-start rounded-lg border px-3 py-2 text-sm transition-colors"
        @click="pickBucket"
      >
        <KunIcon name="lucide:calendar-clock" class="text-default-500 size-4" />
        本月内 · 日期待定
        <span class="bg-primary rounded-full px-1.5 text-xs text-white">
          {{ dayGames.bucket.length }}
        </span>
      </button>
    </div>

    <!-- RIGHT: the focus month's games (normal page flow). -->
    <div class="min-w-0 lg:w-2/3">
      <GalgameCalendarMonthList :data="data" />
    </div>
  </div>
</template>
