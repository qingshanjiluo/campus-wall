<script setup lang="ts">
import {
  KUN_GALGAME_OFFICIAL_CATEGORY_MAP,
  KUN_GALGAME_OFFICIAL_LANGUAGE_MAP,
  KUN_GALGAME_OFFICIAL_LINK_SOURCE_MAP
} from '~/constants/galgameOfficial'

// `id` is a CATALOG LABEL id (A2-3 / doc 106 R1) — a 会社 is a label in the
// registry's vocabulary — carried bare, with no discriminator segment.
//
// The retired `/galgame-official/` space did need one (`/galgame-official/c/{n}`,
// doc 146): there a bare number was a WIKI id, and wiki/catalog id ranges overlap,
// so a single path serving both meanings would silently render the wrong maker.
// `/galgame/official/` never served wiki ids, so the ambiguity cannot arise. The
// old space survives only as 301 shells (server/middleware/legacy-taxonomy.ts) —
// and unlike tag/engine, its wiki→catalog mapping resolves at RUNTIME through the
// registry's external-ref index rather than from a frozen map.
//
// Staff affordances (edit / delete / revision history) are deliberately NOT
// here any more: those write ops address WIKI rows and this page no longer
// knows a wiki id. They live in the admin taxonomy console, which stays
// wiki-ids end to end (doc 106 R11).
const route = useRoute()
const official_id = computed(() => {
  return Number((route.params as { id: string }).id)
})

// A junk segment (/galgame/official/null, crawler-made) becomes NaN and used to
// ride all the way upstream, where the catalog answered 400 — pointless round
// trips for a URL that can only ever be a 404. Answer it here.
if (!Number.isInteger(official_id.value) || official_id.value <= 0) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 会社',
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
// 'nsfw'): the catalog then hides this maker's r18 works from BOTH the list and
// the (content-aware) count, so a NSFW-heavy company can look emptier than it is.
const isSfwMode = computed(() => showKUNGalgameContentLimit.value !== 'nsfw')

// "未发布的游戏": catalog works by this maker that no product has an entry for
// yet. Public claim funnel — open to everyone, not just moderators.
const showDraftsModal = ref(false)

// An unmapped kind falls through to its raw key: a vocabulary that grew
// upstream should look untranslated, not blank (the map used to be indexed
// bare, so a catalog kind with no Chinese entry rendered an empty chip).
const categoryText = (category: string) =>
  KUN_GALGAME_OFFICIAL_CATEGORY_MAP[category] || category
const linkText = (source: string) =>
  KUN_GALGAME_OFFICIAL_LINK_SOURCE_MAP[source] || source

const { data, status } = await useKunFetch<GalgameOfficialDetail>(
  `/galgame-official/${official_id.value}`,
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
      official_id
    }
  }
)

// A merged 会社 keeps its old id addressable, but only as a 301: the catalog
// merges duplicate labels, and the loser's id has to land on the survivor's
// page rather than render a copy of it — two URLs for one company is exactly
// what the redirect prevents. `moved_to` arrives instead of the record, never
// alongside it, so nothing of the survivor is ever painted under the old id.
// The target is built from the shared path builder, so it is the FINAL form
// and the hop can never become a chain.
//
// `navigateTo` is NOT an early return here, awaited or not: on the server it
// only parks a redirect on ssrContext['~renderResponse'] and hands control
// back, so the rest of setup AND the template still render — against the
// tombstone payload (id 0, alias/galgame null). A throw from that render
// preempts the parked redirect and the visitor gets a 500 instead of the 301
// (prod: /galgame/official/13323 died on `alias.length`). So everything below
// that touches the record is gated on `moved`, and the template root carries
// the same gate as `!data.moved_to` — reactive, so a client-side hop stops
// painting the dead brand too.
const moved = !!data.value?.moved_to
if (data.value?.moved_to) {
  await navigateTo(taxonomyDetailPath('official', data.value.moved_to), {
    redirectCode: 301,
    replace: true
  })
}

// An unknown id is a real 404, not an empty 200 shell: this namespace went live
// with no legacy id space behind it, so a miss means the entity does not exist
// and a crawler must be told exactly that rather than indexing a blank page.
if (!data.value) {
  throw createError({
    statusCode: 404,
    statusMessage: '未找到 Galgame 会社',
    fatal: true
  })
}

// A tombstone has no name to describe and no URL of its own to be indexed at —
// the survivor's page owns both.
if (!moved) {
  useKunSeoMeta({
    title: `${data.value.name} 会社`,
    description: `${data.value.name}${data.value.alias?.length ? `, 即 ${data.value.alias.join('| ')}` : ''}, 查看会社 ${data.value.name} 制作的所有 Galgame`
  })
}
</script>

<template>
  <div v-if="data && !data.moved_to" class="space-y-6">
    <KunHeader
      :name="`${data.name} 制作的 Galgame`"
      :description="data.description"
    >
      <template #endContent>
        <div class="space-y-3">
          <p class="text-default-500">
            本页展示本站已收录的、由该会社制作的 Galgame, 可按类型 / 语言 / 平台
            / 排序筛选。本站尚未收录的作品不在此列。默认仅显示 SFW 的 Galgame,
            查看 NSFW Galgame 请在设置面板打开 NSFW 开关。如果有数据错误请
            <KunLink to="/doc/contact"> 联系我们 </KunLink>。
          </p>

          <!-- The 会社's own facts, all of which the payload has always carried
               and none of which this page used to show: its type, the language
               its name is written in, and how many of its games are here. -->
          <div class="text-default-500 flex flex-wrap items-center gap-2">
            会社类别
            <KunChip color="primary">{{ categoryText(data.category) }}</KunChip>
            <template v-if="data.lang">
              名称语言
              <KunChip color="secondary">
                {{ KUN_GALGAME_OFFICIAL_LANGUAGE_MAP[data.lang] || data.lang }}
              </KunChip>
            </template>
            <template v-if="data.galgame_count">
              本站收录
              <KunChip color="success">{{ data.galgame_count }} 部</KunChip>
            </template>
          </div>
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
          <!-- The web presences. Each is named by its own source, so an X
               account is never dressed up as 官方网站 — and a 会社 reachable
               only on X still gets a way to be reached. -->
          <div
            v-if="data.links.length"
            class="text-default-500 flex flex-wrap items-center gap-2"
          >
            会社主页
            <KunLink
              v-for="link in data.links"
              :key="link.url"
              :is-show-anchor-icon="true"
              target="_blank"
              rel="noopener noreferrer"
              underline="hover"
              size="sm"
              :to="link.url"
            >
              {{ linkText(link.source) }}
            </KunLink>
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
      description="当前为 SFW 模式，该会社含 NSFW 内容的 Galgame 不会显示。如需查看，请在设置面板开启 NSFW 开关。"
    />

    <GalgameDraftsModal
      v-model="showDraftsModal"
      entity-type="official"
      :entity-id="official_id"
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
      :description="`${data.name} 会社下暂无 Galgame`"
    />
  </div>
</template>
