<script setup lang="ts">
import { KUN_GALGAME_TAG_CATEGORY_MAP } from '~/constants/galgameTag'

// `id` is a CANONICAL CATALOG tag id (A2-3 / doc 106 R1), carried bare — this
// namespace needs no discriminator segment.
//
// The retired `/galgame-tag/` space did need one (`/galgame-tag/c/{n}`, doc 146):
// there a bare number was a WIKI id, and the two id spaces overlap densely — 718
// of the 1,530 mapped wiki tag ids are themselves live catalog tag ids — so one
// path serving both meanings would silently render the wrong entity for whichever
// one lost. `/galgame/tag/` never served wiki ids at all, so the ambiguity it was
// guarding against cannot arise and the segment is pure baggage. The old space
// survives only as 301 shells (server/middleware/legacy-taxonomy.ts).
//
// Staff affordances (edit / revision history) are deliberately NOT here any
// more: those write ops address WIKI rows and this page no longer knows a wiki
// id. They live in the admin taxonomy console, which stays wiki-ids end to end
// (doc 106 R11).
const route = useRoute()
const tag_id = computed(() => {
  return Number((route.params as { id: string }).id)
})

// A junk segment (/galgame/tag/null, crawler-made) becomes NaN and used to ride
// all the way upstream, where the catalog answered 400 — dozens of pointless
// round trips a day for a URL that can only ever be a 404. Answer it here.
if (!Number.isInteger(tag_id.value) || tag_id.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 标签',
    fatal: true
  })
}

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
// SFW mode mirrors the server's IsSFW (cookie showKUNGalgameContentLimit !==
// 'nsfw'): the catalog then hides r18 works from BOTH the list and the
// (content-aware) count, so an NSFW-heavy entity can look emptier than it is.
const isSfwMode = computed(() => showKUNGalgameContentLimit.value !== 'nsfw')

// "未发布的游戏": catalog works carrying this tag that no product has an entry
// for yet. Public claim funnel — open to everyone, not just moderators.
const showDraftsModal = ref(false)

const { data, status } = await useKunFetch<GalgameTagDetail>(
  `/galgame-tag/${tag_id.value}`,
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
      tag_id
    }
  }
)

// An unknown id is a real 404, not an empty 200 shell: this namespace went live
// with no legacy id space behind it, so a miss means the entity does not exist
// and a crawler must be told exactly that rather than indexing a blank page.
if (!data.value) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 标签',
    fatal: true
  })
}

// Two independent reasons to keep a tag page out of the index: it is adult
// (`category === 'sexual'`), or upstream parked it in the do-not-display tier
// (`hidden` — junk terms, absent from every list, search and picker). Either
// way the page itself still renders: a direct link to a tag always resolves.
const isIndexable = computed(
  () => data.value?.category !== 'sexual' && !data.value?.hidden
)

if (isIndexable.value) {
  useKunSeoMeta({
    title: `标签 ${data.value.name} 的 Galgame`,
    description:
      data.value.description ||
      `含有标签「${data.value.name}」的 Galgame 作品合集, 例如 ${data.value.galgame
        .slice(0, 5)
        .map((g) => getPreferredLanguageText(g.name))
        .join('、')} 等。`,
    ...(data.value.galgame[0]?.banner
      ? { ogImage: data.value.galgame[0].banner }
      : {})
  })
} else {
  useKunDisableSeo(`标签 ${data.value.name} 的 Galgame`)
}
</script>

<template>
  <div v-if="data" class="space-y-6">
    <KunHeader
      :name="`含有标签 ${data.name} 的 Galgame`"
      :description="data.description"
    >
      <template #endContent>
        <div class="space-y-3">
          <p class="text-default-500">
            本页展示本站已收录的、含有该标签的 Galgame, 可按类型 / 语言 / 平台 /
            排序筛选。本站尚未收录的作品不在此列。默认仅显示 SFW 的 Galgame,
            查看 NSFW Galgame 请在设置面板打开 NSFW 开关。如果有数据错误请
            <KunLink to="/doc/contact"> 联系我们 </KunLink>。
          </p>

          <div class="text-default-500">
            标签类别
            <KunChip
              :color="
                data.category === 'content'
                  ? 'primary'
                  : data.category === 'sexual'
                    ? 'danger'
                    : 'success'
              "
            >
              {{ KUN_GALGAME_TAG_CATEGORY_MAP[data.category] }}
            </KunChip>
          </div>
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
      description="当前为 SFW 模式，该标签含 NSFW 内容的 Galgame 不会显示。如需查看，请在设置面板开启 NSFW 开关。"
    />

    <GalgameDraftsModal
      v-model="showDraftsModal"
      entity-type="tag"
      :entity-id="tag_id"
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
      :description="`${data.name} 标签下暂无 Galgame`"
    />
  </div>
</template>
