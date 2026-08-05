<script setup lang="ts">
// `id` is a CATALOG ENGINE id (A2-3 / doc 106 R1), carried bare — this namespace
// needs no discriminator segment.
//
// The retired `/galgame-engine/` space did need one (`/galgame-engine/c/{n}`,
// doc 146): there a bare number was a WIKI id, and the wiki/catalog ranges
// overlap, so one path serving both meanings would silently render the wrong
// engine. `/galgame/engine/` never served wiki ids, so the ambiguity cannot
// arise. The old space survives only as 301 shells — and note that mapping is
// NOT the identity: 52 of the 189 engines landed on a different catalog id
// (server/middleware/legacy-taxonomy.ts).
//
// Staff affordances (edit / delete / revision history) are deliberately NOT
// here any more: those write ops address WIKI rows and this page no longer
// knows a wiki id. They live in the admin taxonomy console, which stays
// wiki-ids end to end (doc 106 R11).
const route = useRoute()
const engine_id = computed(() => {
  return Number((route.params as { id: string }).id)
})

// A junk segment (/galgame/engine/null, crawler-made) becomes NaN and used to
// ride all the way upstream, where the catalog answered 400 — pointless round
// trips for a URL that can only ever be a 404. Answer it here.
if (!Number.isInteger(engine_id.value) || engine_id.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 引擎',
    fatal: true
  })
}

// Shared browse filter Nav with the tag + official detail pages:
// the entity detail lists the forum-LOCAL subset of the engine's catalogue, so
// the same 类型/语言/平台/作品类型 filters + sorts as /galgame apply (backend runs
// them locally over the engine's member ids — see entity_filter.buildEntityFilter).
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

// "未发布的游戏": catalog works built with this engine that no product has an
// entry for yet. Public claim funnel — open to everyone, not just moderators.
const showDraftsModal = ref(false)

const { data, status } = await useKunFetch<GalgameEngineDetail>(
  `/galgame-engine/${engine_id.value}`,
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
      engine_id
    }
  }
)

// An unknown id is a real 404, not an empty 200 shell: this namespace went live
// with no legacy id space behind it, so a miss means the entity does not exist
// and a crawler must be told exactly that rather than indexing a blank page.
if (!data.value) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 引擎',
    fatal: true
  })
}

useKunSeoMeta({
  title: `${data.value.name} 引擎`,
  description: `查看所有使用 ${data.value.name} 引擎制作的 Galgame`
})
</script>

<template>
  <div v-if="data" class="space-y-6">
    <KunHeader
      :name="`${data.name} 引擎制作的 Galgame`"
      :description="data.description"
    >
      <template #endContent>
        <div class="space-y-3">
          <p class="text-default-500">
            本页展示本站已收录的、使用该引擎制作的 Galgame, 可按类型 / 语言 /
            平台 / 排序筛选。本站尚未收录的作品不在此列。默认仅显示 SFW 的
            Galgame, 查看 NSFW Galgame 请在设置面板打开 NSFW
            开关。如果有数据错误请
            <KunLink to="/doc/contact"> 联系我们 </KunLink>。
          </p>

          <div
            v-if="data.alias.length"
            class="text-default-500 flex flex-wrap gap-2"
          >
            别名
            <KunChip
              color="primary"
              v-for="(a, index) in data.alias"
              :key="index"
            >
              {{ a }}
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
      description="当前为 SFW 模式，该引擎含 NSFW 内容的 Galgame 不会显示。如需查看，请在设置面板开启 NSFW 开关。"
    />

    <GalgameDraftsModal
      v-model="showDraftsModal"
      entity-type="engine"
      :entity-id="engine_id"
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
      :description="`${data.name} 引擎下暂无 Galgame`"
    />
  </div>
</template>
