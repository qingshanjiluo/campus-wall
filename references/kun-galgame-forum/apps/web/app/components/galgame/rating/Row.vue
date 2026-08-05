<script setup lang="ts">
// Inline single-rating row — used on the galgame detail page when the
// game only has a handful of ratings (< 3), where the radar chart card
// is overkill and a flat list reads better next to the intro text.
//
// Layout left → right: avatar · name · 「<play_status>了此游戏，
// 表示<recommend>，评分<overall>」. Optional one-line short summary
// truncated to 100 chars below. Click anywhere on the row navigates to
// the rating detail page.
//
// Deliberately NOT wrapped in a KunCard — the parent section already
// frames the rating list, and a stacked-card-in-card look fights the
// rest of the lean detail layout.
import {
  KUN_GALGAME_RATING_RECOMMEND_MAP,
  KUN_GALGAME_RATING_RECOMMEND_COLOR_MAP,
  KUN_GALGAME_RATING_PLAY_STATUS_MAP,
  KUN_GALGAME_RATING_SPOILER_WARNING
} from '~/constants/galgame-rating'

const props = defineProps<{
  rating: GalgameRatingCardOnGalgamePage
}>()

const playStatusLabel = computed(
  () =>
    KUN_GALGAME_RATING_PLAY_STATUS_MAP[props.rating.play_status] ||
    props.rating.play_status
)

const recommendLabel = computed(
  () =>
    KUN_GALGAME_RATING_RECOMMEND_MAP[props.rating.recommend] ||
    props.rating.recommend
)

const recommendColor = computed(() => {
  const c = KUN_GALGAME_RATING_RECOMMEND_COLOR_MAP[props.rating.recommend]
  // text-{color} class — kept as static map so Tailwind JIT picks them up.
  switch (c) {
    case 'danger':
      return 'text-danger'
    case 'success':
      return 'text-success'
    case 'warning':
      return 'text-warning'
    case 'secondary':
      return 'text-secondary'
    default:
      return 'text-default-600'
  }
})

const MAX_SUMMARY = 100

const truncatedSummary = computed(() => {
  const s = props.rating.short_summary?.trim()
  if (!s) return ''
  if (s.length <= MAX_SUMMARY) return s
  return s.slice(0, MAX_SUMMARY) + '...'
})

const overall = computed(() => props.rating.overall.toFixed(1))
</script>

<template>
  <NuxtLink
    :to="`/galgame-rating/${rating.id}`"
    class="hover:bg-default-100/50 group block rounded-md px-2 py-2 transition-colors"
  >
    <div class="flex flex-wrap items-center gap-2 text-sm">
      <KunAvatar :user="rating.user" size="sm" :is-navigation="false" />
      <span class="text-default-800 font-medium">{{ rating.user.name }}</span>
      <span class="text-default-500">
        <template v-if="rating.play_status === 'not_started'">
          还未开始游玩此游戏
        </template>
        <template v-else>
          <span class="text-default-700">{{ playStatusLabel }}</span>
          了此游戏
        </template>
        ，表示
        <span :class="cn('font-medium', recommendColor)">
          {{ recommendLabel }}
        </span>
        ，评分
        <span class="text-default-800 font-semibold">{{ overall }}</span>
      </span>
    </div>

    <!-- Spoiler-flagged ratings hide their summary here; the row links to the
         detail page where it can be read in full. -->
    <p
      v-if="rating.short_summary && rating.spoiler_level !== 'none'"
      class="text-default-500 mt-1 ml-8 flex items-center gap-1 text-sm"
    >
      <KunIcon name="lucide:triangle-alert" class="text-warning shrink-0" />
      {{ KUN_GALGAME_RATING_SPOILER_WARNING }}
    </p>
    <p
      v-else-if="truncatedSummary"
      class="text-default-500 mt-1 ml-8 text-sm"
    >
      {{ truncatedSummary }}
    </p>
  </NuxtLink>
</template>
