<script setup lang="ts">
import type { KunTabItem } from '@kungal/ui-vue'

const props = defineProps<{
  galgame: GalgameDetail
}>()

provide<GalgameDetail>('galgame', props.galgame)

// Resource-publish ban (moderator kill-switch): a live reactive flag the Header
// menu toggles and the resource tab reads for its notice — kept here so both
// stay in sync without mutating the galgame prop.
const resourcePublishBanned = ref(props.galgame.resource_publish_banned)
provide<Ref<boolean>>('galgameResourcePublishBanned', resourcePublishBanned)

// Tab selection is deep-linkable via ?tab=, but a ?comment=<id> notification
// deep-link always wins (it lands on the comment tab + scrolls to the thread).
// Only the always-present tabs are restorable from the URL (patch is async).
const route = useRoute()
const router = useRouter()
const DEEP_LINK_TABS = ['intro', 'resource', 'comment', 'quiz']
const initialTab = () => {
  if (route.query.comment) {
    return 'comment'
  }
  const tab = route.query.tab
  return typeof tab === 'string' && DEEP_LINK_TABS.includes(tab) ? tab : 'intro'
}
const activeTab = ref(initialTab())

// Reflect manual tab switches into the URL (replace, so the back button isn't
// polluted), dropping the one-shot comment deep-link so a reload doesn't re-jump.
watch(activeTab, (tab) => {
  const query = { ...route.query }
  delete query.comment
  delete query.thread
  if (tab !== 'intro' && DEEP_LINK_TABS.includes(tab)) {
    query.tab = tab
  } else {
    delete query.tab
  }
  router.replace({ query })
})
const hasPatchResource = ref(false)

// Per-tab loading, surfaced by each async child, so KunTabPanel :loading can dim
// the panel while its data (re)loads (a 2.10.0 feature). intro is static.
const resourceLoading = ref(false)
const patchLoading = ref(false)
const commentLoading = ref(false)
const quizLoading = ref(false)

const contentTabs = computed<KunTabItem[]>(() => [
  { value: 'intro', textValue: '游戏介绍', icon: 'lucide:book-open' },
  { value: 'resource', textValue: '本体资源下载', icon: 'lucide:download' },
  ...(hasPatchResource.value
    ? [{ value: 'patch', textValue: '补丁资源下载', icon: 'lucide:puzzle' }]
    : []),
  { value: 'comment', textValue: '评论区', icon: 'lucide:messages-square' },
  { value: 'quiz', textValue: '题库', icon: 'lucide:brain' }
])

const ratings = ref([...props.galgame.ratings])
const sortedRatings = computed(() => {
  return [...ratings.value].sort(
    (a, b) => b.short_summary.length - a.short_summary.length
  )
})

const handleRatingCreated = (newRating: GalgameRatingCardOnGalgamePage) => {
  ratings.value.unshift(newRating)
}

// The 贡献者 card is really "创建者 + 贡献者": the creator chip is the wiki-era
// 发布人 (frozen on the local row since the catalog face carries no submitter)
// and stands on its own, so gating the whole card on a non-empty contributor
// list hid the publisher of every game nobody has contributed resources to.
// An unknown creator arrives as the zero user (id 0) — see the API's
// frozenCreatorBrief — so the chip row is gated separately.
const hasCreator = computed(() => props.galgame.user.id > 0)
const hasContributorCard = computed(
  () => hasCreator.value || !!props.galgame.contributor?.length
)
</script>

<template>
  <div class="flex flex-col gap-3">
    <GalgameHeader
      :galgame="galgame"
      @on-rating-created="handleRatingCreated"
    />

    <!-- Wiki-catalogue game the forum hasn't ingested yet (无本地 galgame 行).
         Explains that any interaction records it, the creator/萌萌点 nuances,
         and only offers 认领 on a claimable VNDB draft (status=2) — a published
         entry (status=0) already has a creator and can't be claimed. -->
    <KunInfo
      v-if="galgame.is_on_forum === false"
      color="danger"
      title="该游戏尚未在本站收录"
      description="本站还没有这款 Galgame 的任何本地数据, 当前页面的资料均来自百科。点赞 / 收藏 / 评论 / 评分 都会让它被本站收录, 但您不会成为该 Galgame 的创建者, 也不会获得萌萌点奖励; 发布下载资源同样会让它被收录, 并照常获得发布资源的萌萌点奖励。"
    >
      <p v-if="galgame.status === 2" class="text-sm">
        想成为该 Galgame 的创建者? 可前往「发布
        Galgame」页面认领它（认领后您将成为创建者并获得萌萌点奖励）。
      </p>
    </KunInfo>

    <!-- Mobile: tags sit right under the header. On desktop they live at the top
         of the sidebar instead (the stacked single-column layout would otherwise
         push them below all the main content). Two breakpoint-gated instances —
         see GalgameTag's `variant`. -->
    <div v-if="galgame.tag?.length" class="md:hidden">
      <GalgameTag :tags="galgame.tag" variant="mobile" />
    </div>

    <div
      v-if="sortedRatings.length && sortedRatings.length >= 3"
      class="grid grid-cols-1 gap-3"
    >
      <GalgameRatingRadarCard :ratings="sortedRatings" />
    </div>

    <!-- min-w-0 on both tracks is load-bearing, not tidying. A `1fr` track is
         minmax(AUTO, 1fr): its floor is the item's max-content width, so any
         child wider than its share (a long URL in a comment, a wide table, an
         image arriving without intrinsic size) pushes its own column open and
         steals width from the other one. On a slow connection that happens as
         each tab panel streams in, and the 2/1 split visibly jitters
         side-to-side. min-w-0 lowers the floor to zero and pins the split. -->
    <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
      <div class="order-1 flex min-w-0 flex-col gap-3 md:order-2 md:col-span-2">
        <KunTab
          v-model="activeTab"
          :items="contentTabs"
          variant="solid"
          size="md"
          inner-class-name="bg-[oklch(var(--content1))]!"
        />

        <KunCard
          :is-hoverable="false"
          :is-transparent="false"
          content-class="relative"
        >
          <KunTabPanels v-model="activeTab">
            <KunTabPanel value="intro" class-name="space-y-12">
              <div class="space-y-3">
                <GalgameIntroduction :introduction="galgame.introduction" />

                <div
                  v-if="sortedRatings.length && sortedRatings.length < 3"
                  class="space-y-1"
                >
                  <GalgameRatingRow
                    v-for="rating in sortedRatings"
                    :key="rating.id"
                    :rating="rating"
                  />
                </div>

                <GalgameLink />
              </div>

              <GalgameGallery :screenshots="galgame.screenshots" />

              <GalgameSeriesPanel
                v-if="galgame.series?.length"
                :series="galgame.series"
              />
            </KunTabPanel>

            <KunTabPanel value="resource" :loading="resourceLoading">
              <GalgameResource @update:loading="resourceLoading = $event" />
            </KunTabPanel>

            <KunTabPanel
              v-if="galgame.vndb_id"
              value="patch"
              :loading="patchLoading"
            >
              <GalgamePatchContainer
                :vndb-id="galgame.vndb_id"
                @has-resource="hasPatchResource = $event"
                @update:loading="patchLoading = $event"
              />
            </KunTabPanel>

            <KunTabPanel value="comment" :loading="commentLoading">
              <GalgameCommentCommunityContainer
                @update:loading="commentLoading = $event"
              />
            </KunTabPanel>

            <KunTabPanel value="quiz" :loading="quizLoading">
              <GalgameQuizGalgamePanel @update:loading="quizLoading = $event" />
            </KunTabPanel>
          </KunTabPanels>
        </KunCard>
      </div>

      <div
        class="order-2 flex min-w-0 flex-col gap-3 md:sticky md:top-20 md:order-1 md:col-span-1 md:self-start"
      >
        <div v-if="galgame.tag?.length" class="hidden md:block">
          <GalgameTag :tags="galgame.tag" variant="desktop" />
        </div>

        <GalgameInfo
          :official="galgame.official"
          :engine="galgame.engine"
          :series="galgame.series"
          :age-limit="galgame.age_limit"
          :original-language="galgame.original_language"
          :release-date="galgame.release_date"
          :release-date-tba="galgame.release_date_tba"
        />

        <KunCard
          v-if="hasContributorCard"
          content-class="space-y-3"
          :is-hoverable="false"
          :is-transparent="false"
        >
          <KunHeader
            name="贡献者"
            description="本游戏项目的贡献者, 计 Galgame 资源发布贡献"
            scale="h3"
          />

          <div
            v-if="hasCreator"
            class="text-default-500 flex cursor-default flex-wrap items-center gap-2"
          >
            <KunUserChip :user="galgame.user" />
            <span class="text-sm">
              <KunTime :time="galgame.created" type="date" show-year />
              创建本游戏
            </span>
          </div>

          <GalgameContributorContainer />
        </KunCard>
      </div>
    </div>
  </div>
</template>
