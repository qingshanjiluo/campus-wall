<script setup lang="ts">
import {
  KUN_GALGAME_AGE_LIMIT_MAP,
  getGalgameOriginalLanguageName
} from '~/constants/galgame'

defineProps<{
  galgame: GalgameRatingGalgameInfo
}>()

const getLanguageName = getGalgameOriginalLanguageName
</script>

<template>
  <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
    <div
      className="relative rounded-lg w-full h-full overflow-hidden md:col-span-1 aspect-video"
    >
      <KunImage
        class="size-full rounded-lg object-cover"
        :src="getEffectiveBanner(galgame)"
        loading="lazy"
        :thumbhash="resolveBannerThumbhash(galgame)"
        :alt="getPreferredLanguageText(galgame.name)"
      />
    </div>

    <div class="space-y-3">
      <KunLink :to="`/galgame/${galgame.id}`" underline="none">
        <h1
          class="text-content hover:text-primary text-lg font-bold transition-colors sm:text-2xl"
        >
          {{ `${getPreferredLanguageText(galgame.name)}` }}
        </h1>
      </KunLink>

      <div class="text-default-500 flex items-center gap-3">
        <div class="flex items-center gap-2">
          <KunIcon class-name="text-warning text-2xl" name="lucide:lollipop" />
          <span class="text-warning text-xl font-bold">
            {{
              galgame.rating_count
                ? (galgame.rating / galgame.rating_count).toFixed(1)
                : '0.0'
            }}
          </span>
        </div>
        <span class="bg-default-300 h-3 w-px" />
        <div class="flex items-center gap-2">
          <KunIcon name="lucide:users" />
          <span>{{ galgame.rating_count }} 人评分</span>
        </div>
      </div>

      <div class="flex items-center gap-2">
        <span
          class="text-default-500 dark:text-default-400 text-sm font-medium"
        >
          年龄限制
        </span>
        <KunTooltip
          position="left"
          :text="KUN_GALGAME_AGE_LIMIT_MAP[galgame.age_limit]"
        >
          <KunChip
            variant="flat"
            :color="galgame.age_limit === 'all' ? 'success' : 'danger'"
          >
            {{ galgame.age_limit === 'all' ? '全年龄' : 'R18' }}
          </KunChip>
        </KunTooltip>

        <span class="bg-default-300 h-3 w-px" />

        <span
          class="text-default-500 dark:text-default-400 text-sm font-medium"
        >
          原始语言
        </span>
        <KunChip color="warning" variant="flat">
          {{ getLanguageName(galgame.original_language) }}
        </KunChip>
      </div>

      <dl>
        <GalgameDetailOfficial :official="galgame.official" />
      </dl>
    </div>
  </div>
</template>
