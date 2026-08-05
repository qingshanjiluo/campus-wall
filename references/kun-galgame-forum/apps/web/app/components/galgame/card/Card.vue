<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { GALGAME_RESOURCE_PLATFORM_ICON_MAP } from '~/constants/galgameResource'

const props = defineProps<{
  galgames: GalgameCard[]
  isTransparent?: boolean
  // Max columns at the widest breakpoint. Default 4 (the full-width lists); the
  // calendar's narrower 2/3 panel passes 3 so the cards aren't cramped.
  columns?: 3 | 4
}>()

// Card layout is user-configurable (persisted): each banner corner, the
// NSFW badge, the footer, the secondary JP title, and whether the card opens
// in a new tab — all toggle independently.
const {
  showPlatform,
  showRating,
  showViewLike,
  showLanguage,
  showNsfwBadge,
  showPublisher,
  showJapaneseName,
  isOpenInNewTab
} = storeToRefs(usePersistGalgameCardStore())

const gridClass = computed(() =>
  props.columns === 3
    ? 'grid grid-cols-2 gap-2 sm:grid-cols-2 sm:gap-3 lg:grid-cols-3 xl:grid-cols-3'
    : 'grid grid-cols-2 gap-2 sm:grid-cols-2 sm:gap-3 lg:grid-cols-3 xl:grid-cols-4'
)

// status=2 = unclaimed VNDB draft (calendar). It 404s at /galgame/:gid, so send
// it to the publish wizard pre-searched by name → 认领并发布. Published games
// link to their detail page as usual.
const cardHref = (galgame: GalgameCard) =>
  galgame.status === GalgameStatus.VndbDraft
    ? `/edit/galgame/publish?q=${encodeURIComponent(getPreferredLanguageText(galgame.name))}`
    : `/galgame/${galgame.id}`
</script>

<template>
  <div :class="gridClass">
    <KunCard
      :is-transparent="isTransparent"
      v-for="galgame in galgames"
      :key="galgame.id"
      :href="cardHref(galgame)"
      :target="isOpenInNewTab ? '_blank' : undefined"
      class-name="p-0"
    >
      <div class="relative overflow-hidden">
        <KunImage
          :src="getEffectiveBanner(galgame, { variant: 'mini' })"
          loading="lazy"
          :alt="getPreferredLanguageText(galgame.name)"
          placeholder="/placeholder.webp"
          :thumbhash="resolveBannerThumbhash(galgame)"
          class="h-full w-full object-cover transition-transform duration-300"
          :style="{ aspectRatio: '16/9' }"
        />

        <div
          v-if="
            showPlatform ||
            (showRating && galgame.rating_count) ||
            showNsfwBadge ||
            galgame.is_on_forum === false ||
            galgame.status === GalgameStatus.VndbDraft
          "
          class="absolute top-2 right-2 left-2 flex items-start gap-1"
        >
          <!-- status=2 = 未认领 VNDB 草稿 → 未发布 (the card links to the claim
               wizard). Otherwise a never-ingested wiki game → 未收录. -->
          <span
            v-if="galgame.status === GalgameStatus.VndbDraft"
            class="bg-primary rounded-full px-3 py-1 text-xs whitespace-nowrap text-white backdrop-blur-sm sm:text-sm"
          >
            未在论坛发布
          </span>
          <span
            v-else-if="galgame.is_on_forum === false"
            class="bg-background rounded-full px-3 py-1 text-xs backdrop-blur-sm sm:text-sm"
          >
            未收录
          </span>

          <div v-else-if="showPlatform" class="flex flex-wrap gap-1">
            <template v-if="galgame.platform.length">
              <span
                v-for="(platform, i) in galgame.platform"
                :key="i"
                class="bg-background flex size-6 items-center justify-center rounded-full p-1.5 text-xs backdrop-blur-sm sm:size-8 sm:text-sm"
              >
                <KunIcon
                  :name="GALGAME_RESOURCE_PLATFORM_ICON_MAP[platform]"
                  class="h-4 w-4"
                />
              </span>
            </template>
            <span
              v-else
              class="bg-background rounded-full px-3 py-1 text-xs backdrop-blur-sm sm:text-sm"
            >
              准备中
            </span>
          </div>

          <div class="ml-auto flex flex-col items-end gap-1">
            <span
              v-if="showRating && galgame.rating_count"
              class="bg-background flex items-center gap-1 rounded-full px-2 py-1 text-xs font-medium backdrop-blur-sm sm:text-sm"
            >
              <KunIcon name="lucide:star" class="text-warning" />
              {{ galgame.rating?.toFixed(1) }}
            </span>

            <KunChip
              v-if="showNsfwBadge"
              variant="solid"
              :color="galgame.content_limit === 'sfw' ? 'success' : 'danger'"
            >
              {{ galgame.content_limit.toLocaleUpperCase() }}
            </KunChip>
          </div>
        </div>

        <!-- Sanctioned exception to the no-gradient house rule: a bottom→top
             black scrim so the caption stays legible over the cover image
             (see CLAUDE.md iron rule #2). -->
        <div
          v-if="(showViewLike || showLanguage) && galgame.is_on_forum !== false"
          class="absolute right-0 bottom-0 left-0 flex items-center gap-2 bg-gradient-to-t from-black/60 to-transparent p-2 text-xs transition-opacity duration-300 sm:text-sm"
        >
          <div v-if="showViewLike" class="flex gap-3">
            <span class="flex items-center gap-1">
              <KunIcon class="text-white" name="lucide:eye" />
              <span class="text-white">{{ galgame.view }}</span>
            </span>

            <span class="flex items-center gap-1">
              <KunIcon class="text-white" name="lucide:thumbs-up" />
              <span class="text-white">{{ galgame.like_count }}</span>
            </span>
          </div>

          <div v-if="showLanguage" class="ml-auto flex gap-2">
            <span
              class="text-white"
              v-for="(lang, i) in galgame.language"
              :key="i"
            >
              {{ lang.substring(0, 2).toUpperCase() }}
            </span>
          </div>
        </div>
      </div>

      <div class="flex flex-auto flex-col p-2 sm:p-3">
        <h2
          class="hover:text-primary line-clamp-2 font-medium transition-colors"
        >
          {{ getPreferredLanguageText(galgame.name) }}
        </h2>

        <p
          v-if="
            showJapaneseName &&
            galgame.name['ja-jp'] &&
            galgame.name['ja-jp'] !== getPreferredLanguageText(galgame.name)
          "
          class="text-default-500 mt-1 line-clamp-1 text-sm"
        >
          {{ galgame.name['ja-jp'] }}
        </p>

        <div
          v-if="showPublisher && galgame.is_on_forum !== false"
          class="text-default-600 mt-auto flex items-center gap-1 pt-3 text-sm"
        >
          <KunAvatar
            :disable-floating="true"
            :user="galgame.user"
            size="xs"
            :is-navigation="false"
          />
          {{ galgame.user.name }} ·
          <KunTime :time="galgame.resource_update_time" />
        </div>
      </div>
    </KunCard>
  </div>
</template>
