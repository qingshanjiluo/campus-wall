<script setup lang="ts">
// `id` is a CATALOG SERIES id, carried bare — this namespace never served wiki
// ids, so it needs no discriminator segment (the `/c/` split the retired
// `/galgame-{tag,official,engine}/` spaces needed exists because a bare number
// there was a WIKI id and the two ranges overlap densely).
//
// There is deliberately no legacy redirect shell for series: the wiki's series
// vocabulary was retired outright rather than migrated (6 of its 146 entries
// had a catalog counterpart), so an old /galgame-series/{n} URL addresses
// something that has no successor to point at.
const route = useRoute()
const series_id = computed(() => {
  return Number((route.params as { id: string }).id)
})

// A junk segment (crawler-made /galgame/series/null) becomes NaN and would ride
// all the way upstream for a 400. Answer it here — it can only ever be a 404.
if (!Number.isInteger(series_id.value) || series_id.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 系列',
    fatal: true
  })
}

// Shared browse filter Nav with the tag / official / engine detail pages: the
// entity page lists the forum-LOCAL subset of the series' members, so the same
// 类型/语言/平台/作品类型 filters + sorts as /galgame apply (the backend runs them
// locally over the member ids — entity_filter.buildEntityFilter).
const {
  page,
  limit,
  type,
  language,
  platform,
  gameType,
  sortField,
  sortOrder
} = useGalgameFilters()

const { showKUNGalgameContentLimit } = storeToRefs(usePersistSettingsStore())
// SFW mode mirrors the server's IsSFW: the catalog then hides r18 works from
// BOTH the list and the (content-aware) count, so an NSFW-heavy series can look
// emptier than it is.
const isSfwMode = computed(() => showKUNGalgameContentLimit.value !== 'nsfw')

// "未发布的游戏": catalog members of this series that no product has an entry
// for yet. Public claim funnel — open to everyone, not just moderators.
const showDraftsModal = ref(false)

const { data, status } = await useKunFetch<GalgameSeriesDetail>(
  `/galgame-series/${series_id.value}`,
  {
    method: 'GET',
    query: {
      page,
      limit,
      type,
      language,
      platform,
      gameType,
      sortField,
      sortOrder,
      series_id
    }
  }
)

// An unknown id is a real 404, not an empty 200 shell: this namespace went live
// with no legacy id space behind it, so a miss means the entity does not exist
// and a crawler must be told exactly that rather than indexing a blank page.
if (!data.value) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 系列',
    fatal: true
  })
}

useKunSeoMeta({
  title: `${data.value.name} 系列的 Galgame`,
  description:
    data.value.description ||
    `${data.value.name} 系列收录的 Galgame 作品合集。`
})
</script>

<template>
  <div v-if="data" class="space-y-6">
    <KunHeader
      :name="`${data.name} 系列的 Galgame`"
      :description="data.description"
    >
      <template #endContent>
        <div class="space-y-3">
          <p class="text-default-500">
            本页展示本站已收录的、属于该系列的 Galgame, 可按类型 / 语言 / 平台 /
            排序筛选。本站尚未收录的作品不在此列。默认仅显示 SFW 的 Galgame,
            查看 NSFW Galgame 请在设置面板打开 NSFW 开关。如果有数据错误请
            <KunLink to="/doc/contact"> 联系我们 </KunLink>。
          </p>

          <div class="flex flex-wrap justify-end gap-2">
            <KunButton
              variant="flat"
              color="default"
              @click="showDraftsModal = true"
            >
              <KunIcon name="lucide:library-big" />
              未发布的游戏
            </KunButton>
          </div>
        </div>
      </template>
    </KunHeader>

    <GalgameCardNav :show-advanced="false" />

    <KunInfo
      v-if="isSfwMode"
      color="warning"
      title="部分 Galgame 已隐藏"
      description="当前为 SFW 模式，该系列含 NSFW 内容的 Galgame 不会显示。如需查看，请在设置面板开启 NSFW 开关。"
    />

    <GalgameDraftsModal
      v-model="showDraftsModal"
      entity-type="series"
      :entity-id="series_id"
      :entity-name="data.name"
    />

    <GalgameCard
      :is-transparent="false"
      v-if="data.galgame.length"
      :galgames="data.galgame"
    />

    <KunPagination
      v-if="data.galgame_count > limit"
      v-model:current-page="page"
      :total-page="Math.ceil(data.galgame_count / limit)"
      :is-loading="status === 'pending'"
    />

    <KunNull
      v-if="!data.galgame_count"
      :description="`${data.name} 系列下暂无 Galgame`"
    />
  </div>
</template>
