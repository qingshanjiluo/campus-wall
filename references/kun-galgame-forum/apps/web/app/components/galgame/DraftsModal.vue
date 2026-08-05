<script setup lang="ts">
// "未发布的游戏" overlay launched from an official / tag / engine detail page.
// Lists the unclaimed VNDB drafts (status=2) scoped to THAT entity — games the
// wiki has synced from VNDB but nobody has published on the forum yet. Each row
// renders as a shared GalgameCard whose built-in routing sends status=2 straight
// to the publish wizard (认领并发布). No filter UI by design; the SFW gate is
// applied server-side via the content cookie.
//
// Drafts carry VNDB-synced official + tag edges, so those two scopes are well
// populated; engine edges are human-curated and empty on drafts today, so the
// engine variant honestly shows the empty state.
//
// Pagination is a "加载更多" accumulator (the draft pool can be large), the same
// in-modal pattern as the 萌萌点明细 ledger (MoemoepointLog.vue).
type EntityType = 'official' | 'tag' | 'engine' | 'series'

const props = defineProps<{
  entityType: EntityType
  entityId: number
  entityName: string
}>()

const open = defineModel<boolean>({ required: true })

const LIMIT = 24

// Single source of truth mapping the entity kind to its scoping query param
// (official_id / tag_id / engine_id / series_id) and to its Chinese label.
const ENTITY_LABEL: Record<EntityType, string> = {
  official: '会社',
  tag: '标签',
  engine: '引擎',
  series: '系列'
}
const entityLabel = computed(() => ENTITY_LABEL[props.entityType])
const scopeParam = computed(() => `${props.entityType}_id`)

const items = ref<GalgameCard[]>([])
const total = ref(0)
const page = ref(0)
const status = ref<'idle' | 'loading' | 'loadingMore' | 'error'>('idle')

const hasMore = computed(() => items.value.length < total.value)

const fetchPage = async (more = false) => {
  if (more && (!hasMore.value || status.value === 'loadingMore')) {
    return
  }
  status.value = more ? 'loadingMore' : 'loading'
  const nextPage = more ? page.value + 1 : 1

  const res = await kunFetch<{ items: GalgameCard[]; total: number }>(
    '/galgame/drafts',
    {
      method: 'GET',
      query: {
        page: nextPage,
        limit: LIMIT,
        [scopeParam.value]: props.entityId,
        // Claim-funnel policy: only Japanese / Chinese originals — the raw
        // VNDB pool is ~37% EN/RU/... originals nobody here will claim.
        original_language: 'ja-jp,zh-cn,zh-tw'
      }
    }
  )

  if (res === null) {
    status.value = 'error'
    return
  }

  items.value = more ? [...items.value, ...res.items] : res.items
  total.value = res.total
  page.value = nextPage
  status.value = 'idle'
}

// Fetch fresh on each open (a just-published draft should drop off the list).
watch(open, (isOpen) => {
  if (!isOpen) {
    return
  }
  items.value = []
  total.value = 0
  page.value = 0
  fetchPage(false)
})
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-4xl w-[92vw]">
    <div class="flex max-h-[80dvh] flex-col gap-3 p-1">
      <div class="flex items-center gap-2">
        <KunIcon class="text-primary text-2xl" name="lucide:library-big" />
        <span class="text-lg font-bold">{{ entityName }} 的未发布游戏</span>
        <KunChip v-if="total" size="sm" variant="flat">{{ total }}</KunChip>
      </div>

      <p class="text-default-500 text-xs">
        以下 Galgame 已从 VNDB 收录并归入该{{ entityLabel }}, 但尚未在本站发布,
        点击任意一个即可前往发布页认领并成为创建者。
      </p>

      <KunLoading v-if="status === 'loading'" />

      <KunNull
        v-else-if="status === 'error'"
        description="加载失败, 请稍后再试"
      />

      <KunNull
        v-else-if="!items.length"
        :description="`该${entityLabel}暂无未发布的游戏`"
      />

      <div v-else class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
        <GalgameCard :galgames="items" />

        <div class="flex justify-center pt-1">
          <KunButton
            v-if="hasMore"
            variant="light"
            :loading="status === 'loadingMore'"
            @click="fetchPage(true)"
          >
            加载更多
          </KunButton>
          <span v-else class="text-default-400 text-sm">没有更多了</span>
        </div>
      </div>
    </div>
  </KunModal>
</template>
