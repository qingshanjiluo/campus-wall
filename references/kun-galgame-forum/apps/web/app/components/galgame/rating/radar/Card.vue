<script setup lang="ts">
import {
  KUN_GALGAME_RATING_RECOMMEND_MAP,
  KUN_GALGAME_RATING_RECOMMEND_COLOR_MAP,
  KUN_GALGAME_RATING_SPOILER_MAP,
  KUN_GALGAME_RATING_SPOILER_COLOR_MAP,
  KUN_GALGAME_RATING_SPOILER_WARNING,
  KUN_GALGAME_RATING_PLAY_STATUS_MAP
} from '~/constants/galgame-rating'

defineProps<{
  ratings: GalgameRatingCardOnGalgamePage[]
}>()
</script>

<template>
  <!-- KunScrollShadow 2.5+ real props: `wheel="contain"` = vertical mouse wheel
       scrolls the strip sideways AND keeps the wheel on the strip at its edges so
       the page doesn't move (only while the strip is actually scrollable, so it
       can't freeze the page), `draggable` = click-drag to scroll, `scrollbar="thin"`
       = slim themed bar. Replaces the old dead `scrollbar-visible` class + native
       `:draggable="true"` attr. -->
  <KunScrollShadow
    axis="horizontal"
    shadow-size="5rem"
    wheel="contain"
    draggable
    scrollbar="thin"
  >
    <div class="flex" v-for="rating in ratings" :key="rating.id">
      <KunCard
        :is-transparent="false"
        :is-hoverable="false"
        class-name="max-w-80"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="flex min-w-0 items-center gap-3">
            <KunUserChip
              :disable-floating="true"
              :user="rating.user"
              className="min-w-0 flex-1"
            />
            <span class="text-default-500 shrink-0 text-sm">
              <KunTime :time="rating.created" />
            </span>
          </div>

          <div class="flex items-center gap-2">
            <span
              class="text-warning flex shrink-0 items-center text-xl font-bold"
            >
              {{ `${rating.overall}` }}
              <span class="text-default-500 ml-1.5 text-sm">/ 10 </span>
            </span>
          </div>
        </div>

        <div class="flex gap-2">
          <GalgameRatingRadar
            :model-value="{ ...rating }"
            :size="100"
            :readonly="true"
            label-class="text-[10px]"
          />
          <!-- Spoiler-flagged ratings (portion/serious) hide their summary in
               list contexts; "阅读详情 >" below opens the full review. -->
          <div
            v-if="rating.short_summary && rating.spoiler_level !== 'none'"
            class="text-default-500 flex max-h-[110px] items-center gap-1.5 text-sm"
          >
            <KunIcon name="lucide:triangle-alert" class="text-warning shrink-0" />
            {{ KUN_GALGAME_RATING_SPOILER_WARNING }}
          </div>
          <KunScrollShadow
            v-else-if="rating.short_summary"
            axis="vertical"
            shadow-size="3rem"
            class-name="max-h-[110px]"
            class="text-default-700 text-sm"
          >
            <KunText :content="rating.short_summary" />
          </KunScrollShadow>
        </div>

        <div class="text-default-500 flex flex-wrap items-center gap-2">
          <KunChip
            class-name="shrink-0"
            :color="KUN_GALGAME_RATING_RECOMMEND_COLOR_MAP[rating.recommend]"
          >
            {{ KUN_GALGAME_RATING_RECOMMEND_MAP[rating.recommend] }}
          </KunChip>
          <KunChip color="primary">
            {{ KUN_GALGAME_RATING_PLAY_STATUS_MAP[rating.play_status] }}
          </KunChip>
          <KunChip
            :color="KUN_GALGAME_RATING_SPOILER_COLOR_MAP[rating.spoiler_level]"
          >
            {{ KUN_GALGAME_RATING_SPOILER_MAP[rating.spoiler_level] }}
          </KunChip>
        </div>

        <div class="text-default-500 flex flex-wrap items-center gap-3 text-xs">
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:eye" />
            {{ rating.view }}
          </span>
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:thumbs-up" />
            {{ rating.like_count }}
          </span>
          <KunLink
            :to="`/galgame-rating/${rating.id}`"
            size="sm"
            class-name="ml-auto"
          >
            阅读详情 >
          </KunLink>
        </div>
      </KunCard>
    </div>
  </KunScrollShadow>
</template>
