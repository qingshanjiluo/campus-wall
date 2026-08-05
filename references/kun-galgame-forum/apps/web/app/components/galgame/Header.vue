<script setup lang="ts">
import {
  GALGAME_RESOURCE_TYPE_ICON_MAP,
  GALGAME_RESOURCE_PLATFORM_ICON_MAP
} from '~/constants/galgameResource'
import {
  KUN_GALGAME_RESOURCE_TYPE_MAP,
  KUN_GALGAME_RESOURCE_LANGUAGE_MAP,
  KUN_GALGAME_RESOURCE_PLATFORM_MAP,
  KUN_GALGAME_CONTENT_LIMIT_MAP
} from '~/constants/galgame'

const props = defineProps<{
  galgame: GalgameDetail
}>()

const emits = defineEmits<{
  onRatingCreated: [GalgameRatingCardOnGalgamePage]
}>()

const { id } = usePersistUserStore()
const canBanResourcePublish = useCan('galgame.ban_resource_publish')

// Resource-publish ban (moderator kill-switch): a live reactive flag shared with
// the resource tab (provided by Galgame.vue), so the menu label + tab notice
// update together after a toggle without a refetch.
const resourcePublishBanned = inject<Ref<boolean>>(
  'galgameResourcePublishBanned',
  ref(false)
)
const banning = ref(false)
const toggleResourceBan = async () => {
  if (banning.value) {
    return
  }
  const willBan = !resourcePublishBanned.value
  const ok = await useComponentMessageStore().alert(
    willBan ? '禁止在本游戏下发布资源' : '解除资源发布禁止',
    willBan
      ? '部分游戏可能因为版权方通知，或者其余第三方原因导致不可用，此时需要禁止发布任何下载资源。'
      : '解除后，用户将可以重新在本游戏下发布下载资源。'
  )
  if (!ok) {
    return
  }
  banning.value = true
  const res = await kunFetch(
    `/admin/galgame/${props.galgame.id}/resource-publish-ban`,
    { method: 'PUT', body: { banned: willBan } }
  )
  banning.value = false
  if (res) {
    resourcePublishBanned.value = willBan
    useMessage(willBan ? '已禁止发布资源' : '已解除禁止', 'success')
  }
}

const galgameAliasArray = computed(() => {
  const nameArray = Object.entries(props.galgame.name)
    .filter(
      ([_, value]) => value !== getPreferredLanguageText(props.galgame.name)
    )
    .map(([_, value]) => value)
  return nameArray.concat(props.galgame.alias)
})

const isRatingOpen = ref(false)

// "查看所有封面" modal — only worth offering when there's more than the pinned
// banner cover (covers[] includes the banner at sort_order 0).
const coversOpen = ref(false)
const hasMoreCovers = computed(() => (props.galgame.covers?.length ?? 0) > 1)
</script>

<template>
  <KunCard
    :is-hoverable="false"
    :is-transparent="false"
    content-class="grid grid-cols-1 gap-3 md:grid-cols-3"
  >
    <div
      className="relative rounded-lg w-full h-full overflow-hidden md:col-span-1 aspect-video md:rounded-l-xl"
    >
      <!-- Banner is a real <KunImage>, so use the declarative
           Gallery/Item rather than the document-scan composable.
           wrap=false + v-slot lets the overlay content-limit chip
           stay a sibling that does NOT trigger the lightbox — only
           the image itself opens it. Full-res src (no `mini` variant)
           so the zoomed view is sharp. -->
      <KunLightboxGallery>
        <KunLightboxGalleryItem
          :src="getEffectiveBanner(galgame)"
          :alt="getPreferredLanguageText(galgame.name)"
          :wrap="false"
          v-slot="{ open }"
        >
          <KunImage
            class="size-full cursor-zoom-in object-cover"
            :src="getEffectiveBanner(galgame)"
            loading="eager"
            fetchpriority="high"
            :thumbhash="resolveBannerThumbhash(galgame)"
            :alt="getPreferredLanguageText(galgame.name)"
            @click="open"
          />
        </KunLightboxGalleryItem>
      </KunLightboxGallery>

      <KunChip
        variant="solid"
        class="absolute top-2 left-2"
        :color="galgame.content_limit === 'sfw' ? 'success' : 'danger'"
      >
        <KunTooltip
          position="right"
          :text="KUN_GALGAME_CONTENT_LIMIT_MAP[galgame.content_limit]"
        >
          {{ galgame.content_limit.toLocaleUpperCase() }}
        </KunTooltip>
      </KunChip>

      <!-- 查看所有封面 — sibling of the lightbox (not nested) so it doesn't trigger
           the banner lightbox; opens the covers modal. -->
      <button
        v-if="hasMoreCovers"
        type="button"
        class="bg-background/80 hover:bg-background shadow-kun-sm absolute right-2 bottom-2 z-10 inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium backdrop-blur transition-colors"
        @click="coversOpen = true"
      >
        <KunIcon name="lucide:images" class="size-4" />
        查看所有封面
      </button>
      <GalgameCovers v-model="coversOpen" :covers="galgame.covers" />
    </div>

    <!-- min-w-0: a long unbroken title (Japanese names run wide with no space
         to break on) would otherwise floor this `1fr` track at its max-content
         width and squeeze the banner beside it. -->
    <div className="flex min-w-0 flex-col gap-3 md:col-span-2">
      <div class="flex flex-wrap items-center gap-2">
        <h1 class="text-3xl">
          {{ getPreferredLanguageText(galgame.name) }}
        </h1>
      </div>

      <div class="space-y-3">
        <KunScrollShadow
          axis="vertical"
          shadow-size="2rem"
          class-name="max-h-[100px]"
        >
          <div class="flex flex-wrap gap-2">
            <template v-for="(alias, index) in galgameAliasArray" :key="index">
              <KunChip v-if="alias">{{ alias }}</KunChip>
            </template>
          </div>
        </KunScrollShadow>

        <KunDivider />

        <div class="space-y-1 space-x-1">
          <KunChip
            v-for="(t, index) in galgame.type"
            :key="index"
            color="primary"
          >
            <KunIcon :name="GALGAME_RESOURCE_TYPE_ICON_MAP[t]" />
            {{ KUN_GALGAME_RESOURCE_TYPE_MAP[t] }}
          </KunChip>

          <KunChip
            v-for="(lang, index) in galgame.language"
            :key="index"
            color="secondary"
          >
            <KunIcon class="icon" name="lucide:globe" />
            {{ KUN_GALGAME_RESOURCE_LANGUAGE_MAP[lang] }}
          </KunChip>

          <KunChip
            v-for="(platform, index) in galgame.platform"
            :key="index"
            color="success"
          >
            <KunIcon
              class="icon"
              :name="GALGAME_RESOURCE_PLATFORM_ICON_MAP[platform]"
            />
            {{ KUN_GALGAME_RESOURCE_PLATFORM_MAP[platform] }}
          </KunChip>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <div class="flex items-center gap-1">
            <!-- View count: the same compact pill as 点赞 / 收藏 (KunReaction),
                 but STATIC — action skin (no toggle), no animation, and
                 pointer-events-none so it has no hover / click effect. -->
            <!-- Hidden for not-yet-ingested wiki games: their view is always 0
                 (IncrementView no-ops without a local row), so showing it reads
                 as a misleading stat. The 未收录 notice explains the empty page. -->
            <KunReaction
              v-if="galgame.is_on_forum !== false"
              :count="galgame.view"
              :toggle="false"
              icon="lucide:eye"
              label="浏览量"
              disable-animation
              class="pointer-events-none"
            />

            <GalgameLike
              :galgame-id="galgame.id"
              :target-user-id="galgame.user.id"
              :like-count="galgame.like_count"
              :is-liked="galgame.is_liked"
            />

            <GalgameFavorite
              :galgame-id="galgame.id"
              :target-user-id="galgame.user.id"
              :favorite-count="galgame.favorite_count"
              :is-favorited="galgame.is_favorited"
            />
          </div>

          <!-- ml-auto keeps this group right-aligned even after it wraps to its
               own line on mobile (justify-between would left-align a wrapped row). -->
          <div class="ml-auto flex flex-wrap items-center justify-end gap-1">
            <KunButton
              variant="shadow"
              color="primary"
              size="sm"
              @click="isRatingOpen = true"
            >
              <span class="flex items-center gap-1">
                <KunIcon name="lucide:star" />添加评分
              </span>
            </KunButton>

            <!-- 正版购买 (DLsite affiliate). Only for galgames that actually
                 resolve to a DLsite work; the component owns the coupon-then-buy
                 popover. `flat` rather than another `shadow`: two solid primaries
                 side by side would fight, and 添加评分 stays this page's own
                 primary action. -->
            <GalgameDlsitePurchase
              v-if="galgame.dlsite_purchase_url"
              :purchase-url="galgame.dlsite_purchase_url"
              :coupon-url="galgame.dlsite_coupon_url"
            />

            <!-- The schema-driven proposal editor (the engine review queue).
                 The legacy rewrite editor retired in E3b. -->
            <KunButton
              variant="light"
              color="default"
              size="sm"
              @click="navigateTo(`/galgame/${galgame.id}/edit`)"
            >
              <span class="flex items-center gap-1">
                <KunIcon name="lucide:file-pen-line" />编辑资料
              </span>
            </KunButton>

            <KunPopover
              v-if="galgame.user.id !== id || canBanResourcePublish"
              position="bottom-end"
            >
              <template #trigger>
                <KunButton
                  :is-icon-only="true"
                  variant="light"
                  color="default"
                  size="sm"
                >
                  <KunIcon name="lucide:ellipsis" />
                </KunButton>
              </template>
              <div class="flex w-44 flex-col gap-1 p-2">
                <ReportButton
                  v-if="galgame.user.id !== id"
                  menu
                  subject-kind="galgame"
                  :subject-id="galgame.id"
                  :snapshot="getPreferredLanguageText(galgame.name)"
                  :subject-url="`${kungal.domain.main}/galgame/${galgame.id}`"
                />
                <!-- Moderator kill-switch: forbid / allow publishing download
                     resources under this game (copyright / third-party). -->
                <KunButton
                  v-if="canBanResourcePublish"
                  variant="light"
                  :color="resourcePublishBanned ? 'success' : 'danger'"
                  size="sm"
                  class-name="w-full justify-start gap-2"
                  :loading="banning"
                  @click="toggleResourceBan"
                >
                  <KunIcon
                    :name="
                      resourcePublishBanned
                        ? 'lucide:circle-check'
                        : 'lucide:ban'
                    "
                  />
                  {{
                    resourcePublishBanned ? '解除资源发布禁止' : '禁止发布资源'
                  }}
                </KunButton>
              </div>
            </KunPopover>

            <GalgameRatingPublish
              v-model="isRatingOpen"
              :galgame-id="galgame.id"
              @on-published="(newRating) => emits('onRatingCreated', newRating)"
            />
          </div>
        </div>
      </div>
    </div>
  </KunCard>
</template>
