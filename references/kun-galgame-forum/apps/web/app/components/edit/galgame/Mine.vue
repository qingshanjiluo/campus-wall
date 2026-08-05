<script setup lang="ts">
// "我的提交" — the works whose lifecycle the current user moved, served by
// GET /api/galgame/mine off the registry's per-user claim face.
//
// Each row carries the work's LATEST transition by anyone, which is why a
// decline reason shows up here without a second request: what a submitter needs
// on their own submission is the reviewer's verdict, an event they did not
// cause.
//
// This is the post-submit landing page — a submission awaiting review has no
// public page to redirect to.
//
// TEMPLATE SINGLE-ROOT: the root <KunCard> renders unconditionally and
// must be the *only* top-level template node — NOT preceded by a
// template comment. In dev, Vue keeps template comments as comment
// vnodes, so a leading `<!-- ... -->` becomes a sibling of KunCard,
// making the page's render root a 2-child Fragment and tripping Nuxt's
// "does not have a single root node" route-transition warning. A bare
// `v-if="data"` root has the same effect (empty comment vnode on
// fetch failure). Hence: all conditional content lives INSIDE the card.

// The face is cursor-paged, not offset-paged: this list is being written to
// while it is read, and a cursor is what makes paging neither skip nor repeat a
// row. `before` is the previous page's next_before.
const pageData = reactive({
  before: 0,
  limit: 20
})

const { data, status, refresh } = await useKunFetch<UserClaimList>(
  '/galgame/mine',
  { query: pageData }
)

// State badge is shared (shared/utils/galgameClaimState.ts) so this list, the
// wizard, the draft editor and the moderation queue cannot drift apart.
const stateBadge = galgameClaimStateBadge
const nameOf = (item: UserClaimItem) => item.display_name || '(无标题)'
// product_work_id IS the gid; a work with no anchor has no kungal page.
const gidOf = (item: UserClaimItem) => item.product_work_id ?? 0

const isWithdrawing = ref<Record<number, boolean>>({})

const handleWithdraw = async (item: UserClaimItem) => {
  const ok = await useComponentMessageStore().alert(
    '确定撤回这条申请吗?',
    '撤回后该条目会退回草稿状态, 不再公开展示。您填写的内容不会丢失, 随时可以重新提交。'
  )
  if (!ok) {
    return
  }
  const gid = gidOf(item)
  isWithdrawing.value = { ...isWithdrawing.value, [gid]: true }
  const res = await kunFetch<string>(`/galgame/${gid}`, {
    method: 'DELETE'
  })
  isWithdrawing.value = { ...isWithdrawing.value, [gid]: false }
  if (res !== null) {
    useMessage('已撤回', 'success')
    refresh()
  }
}

// A declined submission goes back to the queue without being re-typed — the
// content is already on the entry, only its state moves.
const isResubmitting = ref<Record<number, boolean>>({})

const handleResubmit = async (item: UserClaimItem) => {
  const gid = gidOf(item)
  isResubmitting.value = { ...isResubmitting.value, [gid]: true }
  const res = await kunFetch<unknown>(`/galgame/${gid}/resubmit`, {
    method: 'POST'
  })
  isResubmitting.value = { ...isResubmitting.value, [gid]: false }
  if (res !== null) {
    useMessage('已重新提交审核', 'success')
    refresh()
  }
}
</script>

<template>
  <div class="space-y-4">
    <KunHeader
      name="我的 Galgame 提交"
      description="您提交的 Galgame 申请, 审核中 / 已拒绝 / 草稿都会显示在此处。审核通过的 Galgame 会成为公开条目, 不再列在这里。"
    >
      <template #endContent>
        <div class="flex gap-2">
          <KunLink to="/edit/galgame/publish">
            <KunButton size="sm">新建提交</KunButton>
          </KunLink>
          <KunLink to="/message/notice">
            <KunButton size="sm" variant="flat">审核通知</KunButton>
          </KunLink>
        </div>
      </template>
    </KunHeader>

    <KunDivider />

    <KunInfo
      v-if="!data"
      color="danger"
      title="加载失败"
      description="无法获取您的提交列表, 可能是后端 / Galgame 资料库暂时不可用, 请稍后重试。"
    />

    <div v-else-if="data.items.length" class="flex flex-col gap-3">
      <div
        v-for="item in data.items"
        :key="item.work_id"
        class="dark:border-default-200 flex flex-col gap-3 rounded-lg border border-transparent p-3 backdrop-blur-none transition-all duration-200 sm:flex-row sm:items-center"
      >
        <div class="min-w-0 flex-1 space-y-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3
              class="hover:text-primary truncate text-lg font-medium transition-colors"
            >
              {{ nameOf(item) }}
            </h3>
            <KunChip
              size="xs"
              variant="flat"
              :color="stateBadge(item.claim_state).color"
            >
              {{ stateBadge(item.claim_state).label }}
            </KunChip>
          </div>
          <div class="text-default-500 flex flex-wrap items-center gap-2 text-sm">
            <span>提交于 <KunTime :time="item.first_acted_at" /></span>
            <template v-if="item.last_event_at !== item.first_acted_at">
              <span>·</span>
              <span>最后处理 <KunTime :time="item.last_event_at" /></span>
            </template>
          </div>
          <div
            v-if="item.claim_state === CLAIM_STATE_DECLINED && item.last_reason"
            class="text-danger bg-danger/10 mt-1 rounded-md px-2 py-1 text-sm"
          >
            被拒原因: {{ item.last_reason }}
          </div>
        </div>
        <div class="flex shrink-0 gap-2">
          <KunLink :to="`/galgame/${gidOf(item)}/edit`">
            <KunButton size="sm" variant="flat">编辑</KunButton>
          </KunLink>
          <KunButton
            v-if="item.claim_state === CLAIM_STATE_DECLINED"
            size="sm"
            color="primary"
            variant="flat"
            :loading="isResubmitting[gidOf(item)]"
            :disabled="isResubmitting[gidOf(item)]"
            @click="handleResubmit(item)"
          >
            重新提交
          </KunButton>
          <KunButton
            v-else-if="item.claim_state !== CLAIM_STATE_DRAFT"
            size="sm"
            color="danger"
            variant="flat"
            :loading="isWithdrawing[gidOf(item)]"
            :disabled="isWithdrawing[gidOf(item)]"
            @click="handleWithdraw(item)"
          >
            撤回
          </KunButton>
        </div>
      </div>
    </div>

    <KunNull v-else-if="data && !data.items.length" />

    <KunButton
      v-if="data && data.next_before"
      variant="flat"
      :loading="status === 'pending'"
      @click="pageData.before = data.next_before"
    >
      加载更多
    </KunButton>
  </div>
</template>
