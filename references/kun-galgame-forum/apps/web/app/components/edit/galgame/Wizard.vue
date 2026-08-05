<script setup lang="ts">
// Publish wizard — the first stop in the "发布 Galgame" flow. Goal: keep
// duplicate submissions out of the moderation queue by surfacing existing
// records before the user fills out a full form.
//
// Two paths only (VNDB-id precise lookup was removed — the registry holds the
// full VNDB set as unpublished drafts, so a name search already surfaces them;
// users don't need to know VNDB ids):
//   1. Search by name → resolve to one of:
//        live     → 前往发布资源 (/galgame/:gid)
//        draft    → 认领并发布 (POST /:gid/claim, +3 萌萌点)
//        pending  → 他人投稿审核中, no action (see below)
//   2. Nothing matches → 新建申请 → /edit/galgame/create
//
// Both halves are the registry's. `pending` is the caller's own backlog off the
// per-user claim face; `items` is the registry search with
// claim_state=live,draft,pending.
//
// `pending` in that list is the one worth explaining: it is somebody ELSE's
// submission awaiting review. The wizard has to SHOW it — an entry it hides is
// an entry that gets submitted twice, which is the whole failure this screen
// exists to prevent — but 认领 on it would be refused, so the row is labelled
// 审核中 and offers no action. Until the projector separates the two, such rows
// arrive as `draft` instead and behave as they always did: the 认领 attempt is
// what discovers the difference.

interface SearchHit {
  id: number
  vndb_id?: string
  name_zh_cn?: string
  name_ja_jp?: string
  name_en_us?: string
  name_zh_tw?: string
  banner?: string
  effective_banner_hash?: string
  claim_state?: string
}

interface WizardSearchResp {
  items: SearchHit[]
  pending?: UserClaimItem[]
  total: number
}

const q = ref('')
const hasSearched = ref(false)
const isSearching = ref(false)
const searchResults = ref<WizardSearchResp | null>(null)

// State badge + wire-name resolution are shared (shared/utils/
// galgameClaimState.ts). Fallback (VNDB id / #id) computed per call site.
const nameOfHit = (h: SearchHit): string =>
  galgameNameFromWire(h, h.vndb_id ? `VNDB ${h.vndb_id}` : `#${h.id}`)

const stateBadge = galgameClaimStateBadge
const gidOfPending = (item: UserClaimItem) => item.product_work_id ?? 0

const handleSearch = async () => {
  if (!q.value.trim()) {
    useMessage('请先输入关键词', 'warn')
    return
  }
  isSearching.value = true
  // The session identifies the caller, which is what makes the `pending` half
  // personal — it is their own claim history, not a filter over the search.
  const res = await kunFetch<WizardSearchResp>('/galgame/search/wizard', {
    method: 'GET',
    query: { q: q.value.trim(), limit: 12 }
  })
  isSearching.value = false
  hasSearched.value = true
  searchResults.value = res
}

const isClaiming = ref(false)

const handleClaim = async (gid: number) => {
  const ok = await useComponentMessageStore().alert(
    '认领此草稿吗?',
    '认领后该条目立即变为已发布状态, 您将成为该 Galgame 的创建者, 并获得 +3 萌萌点。若该条目其实是他人正在审核中的投稿, 认领会被拒绝。'
  )
  if (!ok) return

  isClaiming.value = true
  const result = await kunFetch<{ to_state: string }>(
    `/galgame/${gid}/claim`,
    { method: 'POST', body: {} }
  )
  isClaiming.value = false
  if (result?.to_state) {
    useKunLoliInfo('认领成功, 已发布', 5)
    await navigateTo(`/galgame/${gid}`)
  }
}

// Carry the typed name over to the create form so the user doesn't
// re-type it.
const handleCreateNew = async () => {
  const store = usePersistEditGalgameStore()
  if (q.value.trim() && !store.name['zh-cn']) {
    store.name['zh-cn'] = q.value.trim()
  }
  await navigateTo('/edit/galgame/create')
}

const noMatches = computed(
  () =>
    hasSearched.value &&
    searchResults.value !== null &&
    !searchResults.value.items.length &&
    !searchResults.value.pending?.length
)

// Arrived from a calendar "未发布" card (/edit/galgame/publish?q=<name>):
// pre-fill + auto-search so the clicked draft surfaces straight away for 认领.
const route = useRoute()
onMounted(() => {
  const pre = route.query.q
  if (typeof pre === 'string' && pre.trim()) {
    q.value = pre.trim()
    handleSearch()
  }
})
</script>

<template>
  <div class="space-y-6">
    <KunHeader
      name="发布 Galgame"
      description="先搜索您想发布的游戏：已存在的直接前往或一键认领，确实没有的再新建申请，避免重复提交。"
    >
      <template #endContent>
        <KunLink to="/edit/galgame/mine">
          <KunButton size="sm" variant="flat">我的提交</KunButton>
        </KunLink>
      </template>
    </KunHeader>

    <KunDivider>
      <span class="mx-2">① 搜索是否已存在</span>
    </KunDivider>

    <div class="space-y-2">
      <div class="flex items-center gap-2">
        <KunInput
          v-model="q"
          placeholder="输入游戏名 (任意语言)"
          @keydown.enter="handleSearch"
        />
        <KunButton
          class-name="whitespace-nowrap"
          :loading="isSearching"
          @click="handleSearch"
        >
          搜索
        </KunButton>
      </div>
      <p class="text-default-500 text-sm">
        搜索覆盖已发布的 Galgame、尚未发布的草稿 (可一键认领), 以及他人正在审核中的投稿
        (标记为「审核中」, 无法认领); 同时会显示您自己的待审核 / 已拒绝投稿。
      </p>
    </div>

    <div v-if="searchResults" class="space-y-4">
      <!-- pending: the caller's OWN backlog, most actionable, shown first -->
      <div
        v-if="searchResults.pending && searchResults.pending.length"
        class="space-y-2"
      >
        <h3 class="text-default-700 text-sm font-bold">您的待审 / 已拒草稿</h3>
        <div
          v-for="item in searchResults.pending"
          :key="`pending-${item.work_id}`"
          class="dark:border-default-200 flex flex-col gap-3 rounded-lg border border-transparent p-3 backdrop-blur-none transition-all duration-200 sm:flex-row sm:items-center"
        >
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="truncate font-medium">
                {{ item.display_name || `#${item.work_id}` }}
              </h4>
              <KunChip
                size="xs"
                variant="flat"
                :color="stateBadge(item.claim_state).color"
              >
                {{ stateBadge(item.claim_state).label }}
              </KunChip>
            </div>
            <p v-if="item.last_reason" class="text-default-500 text-sm">
              {{ item.last_reason }}
            </p>
          </div>
          <KunLink :to="`/galgame/${gidOfPending(item)}/edit`">
            <KunButton size="sm" variant="flat">继续编辑</KunButton>
          </KunLink>
        </div>
      </div>

      <!--
        items: search hits. The registry surfaces unpublished entries too, so
        the action MUST branch on claim_state:
          live    → 前往发布资源
          draft   → 认领并发布 (a draft has no public page; a blanket detail
                    link would 404)
          pending → somebody else's submission under review: shown so it is not
                    submitted twice, but there is nothing this user may do to it
      -->
      <div v-if="searchResults.items.length" class="space-y-2">
        <h3 class="text-default-700 text-sm font-bold">匹配的 Galgame</h3>
        <div
          v-for="hit in searchResults.items"
          :key="`item-${hit.id}`"
          class="dark:border-default-200 flex flex-col gap-3 rounded-lg border border-transparent p-3 backdrop-blur-none transition-all duration-200 sm:flex-row sm:items-center"
        >
          <KunImage
            v-if="hit.banner"
            :src="hit.banner"
            loading="lazy"
            placeholder="/placeholder.webp"
            class="h-16 w-28 shrink-0 rounded object-cover"
            :style="{ aspectRatio: '16/9' }"
          />
          <div class="min-w-0 flex-1 space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <h4 class="truncate font-medium">{{ nameOfHit(hit) }}</h4>
              <KunChip
                size="xs"
                variant="flat"
                :color="stateBadge(hit.claim_state).color"
              >
                {{ stateBadge(hit.claim_state).label }}
              </KunChip>
            </div>
            <p class="text-default-500 text-sm">
              VNDB: {{ hit.vndb_id || '—' }}
            </p>
          </div>
          <KunButton
            v-if="isClaimableState(hit.claim_state)"
            size="sm"
            :loading="isClaiming"
            :disabled="isClaiming"
            @click="handleClaim(hit.id)"
          >
            认领并发布
          </KunButton>
          <span
            v-else-if="hit.claim_state === CLAIM_STATE_PENDING"
            class="text-default-400 shrink-0 text-sm"
          >
            他人投稿审核中
          </span>
          <KunLink v-else :to="`/galgame/${hit.id}`">
            <KunButton size="sm" variant="flat">前往发布资源</KunButton>
          </KunLink>
        </div>
      </div>

      <KunInfo
        v-if="noMatches"
        color="info"
        title="没有找到匹配的 Galgame"
        description="确认确实没有后, 用下方「新建 Galgame 申请」提交。"
      />
    </div>

    <KunDivider>
      <span class="mx-2">② 都没有？新建申请</span>
    </KunDivider>

    <KunInfo
      color="info"
      title="提交一份新的 Galgame 申请"
      description="仅用于 VNDB 未收录的原创 / 同人 / 独立作品。提交后进入审核队列, 审核通过才会公开。"
    />
    <div class="flex justify-end">
      <KunButton size="lg" @click="handleCreateNew">新建 Galgame 申请</KunButton>
    </div>
  </div>
</template>
