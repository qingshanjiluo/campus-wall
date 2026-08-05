<script setup lang="ts">
// Hydration-safe, timezone-aware timestamp.
//
// The problem this solves:
//   - `formatTimeDifference` uses `new Date()` (now) → the server renders with
//     its render-time `now`, the client re-renders during hydration with
//     `now + network latency` → the relative string can differ → hydration
//     mismatch (looks like the page "loads twice").
//   - `formatDate` formats in the *runtime's* timezone. The SSR runtime is not
//     the viewer's timezone, so an absolute time renders in the server zone on
//     first paint (wrong for the viewer) AND mismatches the client render.
//
// The fix:
//   - The server computes the string ONCE and ships it in the Nuxt payload via
//     `useState` (keyed by a stable `useId`). The client hydrates from that
//     exact same string → no mismatch, ever.
//   - After mount we recompute with the browser's own timezone (date-fns
//     `format` reads the runtime TZ) and a live `now`, so each viewer sees
//     their LOCAL time, and relative stamps keep ticking. `data-allow-mismatch`
//     is a belt-and-suspenders for any residual (e.g. nested Suspense) case.
const props = withDefaults(
  defineProps<{
    time: number | string | Date | null | undefined
    // 'relative' → "X 分钟前" | 'date' → MM-dd | 'datetime' → MM-dd - HH:mm |
    // 'auto' → relative within a day, otherwise a precise date (MM-dd - HH:mm)
    type?: 'relative' | 'date' | 'datetime' | 'auto'
    // include the year in 'date' / 'datetime' modes (yyyy-MM-dd)
    showYear?: boolean
  }>(),
  { type: 'relative', showYear: false }
)

const isWithinDay = (value: number | string | Date): boolean => {
  const date = new Date(value)
  return (
    !Number.isNaN(date.getTime()) && Date.now() - date.getTime() < 86_400_000
  )
}

const render = (): string => {
  const value = props.time ?? ''
  if (
    props.type === 'relative' ||
    (props.type === 'auto' && value !== '' && isWithinDay(value))
  ) {
    return formatTimeDifference(value)
  }
  return formatDate(value, {
    isShowYear: props.showYear,
    isPrecise: props.type === 'datetime' || props.type === 'auto'
  })
}

const text = useState(`kun-time-${useId()}`, render)

let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  // Recompute in the viewer's timezone / with a live `now`.
  text.value = render()
  if (props.type === 'relative' || props.type === 'auto') {
    timer = setInterval(() => {
      text.value = render()
    }, 60_000)
  }
})
onBeforeUnmount(() => {
  if (timer) {
    clearInterval(timer)
  }
})

// Machine-readable, timezone-explicit — identical on both sides (never
// mismatches) and good for SEO / accessibility.
const machineDateTime = computed(() => {
  if (props.time === null || props.time === undefined || props.time === '') {
    return undefined
  }
  const date = new Date(props.time)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
})
</script>

<template>
  <time class="text-default-500" :datetime="machineDateTime" data-allow-mismatch>{{ text }}</time>
</template>
