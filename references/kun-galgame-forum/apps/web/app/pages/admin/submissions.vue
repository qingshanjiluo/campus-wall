<script setup lang="ts">
// Galgame submission review queue: the entries whose claim is `pending`, with
// the four verdicts.
//
// It used to list MESSAGES — a log of things that had happened, read as a
// worklist by convention — which meant the queue and reality could disagree.
// This lists the CLAIMS themselves, so an entry leaves the queue because its
// state moved and for no other reason. There is no "mark as handled" because
// there is nothing to mark.
//
// Moderator+ (moderator ⊂ admin ⊂ ren): mirrors the API's RequireModerator
// entry gate. UX guard only — the real boundary is the API, and behind it the
// registry re-checks the acting user against catalog.claim.review.
definePageMeta({ middleware: 'moderator' })

useKunDisableSeo('Galgame 审核')

// One pending entry, as the registry's works list renders it.
interface PendingClaim {
  id: number
  display_name: string
  // claimed_by.work_id IS the gid: the id kungal told the registry to record,
  // and therefore the id every action and link here is keyed by. The registry
  // work id is NOT interchangeable with it — the two ranges overlap.
  claimed_by: { site: string; work_id: number; state: string } | null
  names?: Record<string, string>
  updated?: string
}

interface PendingQueueEnvelope {
  items: PendingClaim[]
  next_cursor: string | null
}

// The queue reads the LIVE list, not the search index: a search index reflects
// a claim-state change only at the next rebuild, and a moderation queue showing
// entries somebody approved an hour ago is worse than no queue.
const pageData = reactive({
  cursor: '',
  limit: 30
})

const { data, status, refresh } = await useKunFetch<PendingQueueEnvelope>(
  '/admin/galgame/submissions',
  { query: pageData }
)

const gidOf = (row: PendingClaim) => row.claimed_by?.work_id ?? 0

// Row title: the entry's localized name, falling back to the registry's own
// display name and then to the gid.
const displayName = (row: PendingClaim): string => {
  const localized = row.names
    ? getPreferredLanguageText({
        'en-us': row.names['en-us'] ?? '',
        'ja-jp': row.names['ja-jp'] ?? '',
        'zh-cn': row.names['zh-cn'] ?? '',
        'zh-tw': row.names['zh-tw'] ?? ''
      })
    : ''
  return localized || row.display_name || `#${gidOf(row)}`
}

const isActing = ref<Record<number, boolean>>({})

// Reason-capture modal state. KunUI doesn't have a built-in prompt
// dialog and useComponentMessageStore only exposes a confirm-style
// `alert`, so we inline a small KunModal + KunTextarea here for the
// two flows that need a free-text reason (decline / ban).
type ReasonAction = 'decline' | 'ban'

interface ReasonContext {
  action: ReasonAction
  target: PendingClaim
}

const isReasonModalOpen = ref(false)
const reasonContext = ref<ReasonContext | null>(null)
const reasonText = ref('')

const modalTitle = computed(() => {
  if (!reasonContext.value) return ''
  const name = displayName(reasonContext.value.target)
  return reasonContext.value.action === 'decline'
    ? `拒绝《${name}》`
    : `封禁《${name}》`
})

const modalDescription = computed(() => {
  if (!reasonContext.value) return ''
  return reasonContext.value.action === 'decline'
    ? '请填写拒绝原因, 提交者会在站内消息中看到。'
    : '封禁后该条目对所有人不可见。如不填写理由, 仅记录管理员操作。'
})

const modalRequiresReason = computed(
  () => reasonContext.value?.action === 'decline'
)

const openReasonModal = (action: ReasonAction, target: PendingClaim) => {
  reasonContext.value = { action, target }
  reasonText.value = ''
  isReasonModalOpen.value = true
}

const closeReasonModal = () => {
  isReasonModalOpen.value = false
  // Clear the context after the leave-transition so the title/desc
  // computeds don't blink to empty mid-animation.
  setTimeout(() => {
    reasonContext.value = null
    reasonText.value = ''
  }, 300)
}

// One verdict. The action IS the vocabulary — there is no status integer to
// PUT — so "who decided this and why" is recorded by the write itself rather
// than by a reviewer remembering to note it somewhere.
const applyVerdict = async (
  row: PendingClaim,
  action: 'approve' | 'decline' | 'ban',
  reason: string
) => {
  const gid = gidOf(row)
  isActing.value = { ...isActing.value, [gid]: true }
  const res = await kunFetch<unknown>(`/admin/galgame/${gid}/review`, {
    method: 'POST',
    body: { action, reason }
  })
  isActing.value = { ...isActing.value, [gid]: false }
  if (res !== null) {
    useMessage('已处理', 'success')
    refresh()
  }
}

const handleApprove = async (row: PendingClaim) => {
  const ok = await useComponentMessageStore().alert(
    `通过《${displayName(row)}》?`,
    '通过后该 Galgame 立即公开发布, 提交者会收到通知并自动获得 +3 萌萌点 (由资料库事件同步发放)。'
  )
  if (!ok) return
  await applyVerdict(row, 'approve', '')
}

const handleConfirmReason = async () => {
  const ctx = reasonContext.value
  if (!ctx) return
  const trimmed = reasonText.value.trim()
  if (modalRequiresReason.value && !trimmed) {
    useMessage('拒绝原因不能为空', 'warn')
    return
  }
  closeReasonModal()
  await applyVerdict(ctx.target, ctx.action, trimmed)
}
</script>

<template>
  <div v-if="data" class="w-full space-y-4">
    <KunHeader
      name="Galgame 审核"
      description="审核用户提交的新 Galgame。通过后立即公开发布并向提交者发放 +3 萌萌点; 拒绝时需说明原因, 提交者可据此修改后重新提交。"
    />

    <KunDivider />

    <div v-if="data.items.length" class="flex flex-col gap-3">
      <div
        v-for="row in data.items"
        :key="row.id"
        class="dark:border-default-200 flex flex-col gap-3 rounded-lg border border-transparent p-3 backdrop-blur-none transition-all duration-200 sm:flex-row sm:items-start"
      >
        <div class="min-w-0 flex-1 space-y-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="truncate text-lg font-medium">
              {{ displayName(row) }}
            </h3>
            <KunChip
              size="xs"
              variant="flat"
              :color="galgameClaimStateBadge(row.claimed_by?.state).color"
            >
              {{ galgameClaimStateBadge(row.claimed_by?.state).label }}
            </KunChip>
          </div>
          <div
            class="text-default-500 flex flex-wrap items-center gap-2 text-sm"
          >
            <span v-if="row.updated"><KunTime :time="row.updated" /></span>
            <span v-if="row.updated">·</span>
            <span>galgame_id: {{ gidOf(row) }}</span>
          </div>
        </div>
        <div class="flex shrink-0 flex-wrap gap-2">
          <!-- Preview the entry as a reader sees it, not the proposal editor:
               a reviewer decides on the PAGE (cover, screenshots, intro,
               links), and the detail header carries 编辑资料 for the cases
               that need a fix before approval. A pending claim renders as a
               draft here — only `hidden` is withheld from the detail face. -->
          <KunLink :to="`/galgame/${gidOf(row)}`" target="_blank">
            <KunButton size="sm" variant="flat">查看</KunButton>
          </KunLink>
          <KunButton
            size="sm"
            color="success"
            :loading="isActing[gidOf(row)]"
            :disabled="isActing[gidOf(row)]"
            @click="handleApprove(row)"
          >
            通过
          </KunButton>
          <KunButton
            size="sm"
            color="danger"
            variant="flat"
            :loading="isActing[gidOf(row)]"
            :disabled="isActing[gidOf(row)]"
            @click="openReasonModal('decline', row)"
          >
            拒绝
          </KunButton>
          <KunButton
            size="sm"
            color="default"
            variant="light"
            :loading="isActing[gidOf(row)]"
            :disabled="isActing[gidOf(row)]"
            @click="openReasonModal('ban', row)"
          >
            封禁
          </KunButton>
        </div>
      </div>
    </div>

    <KunNull v-if="!data.items.length" />

    <KunButton
      v-if="data.next_cursor"
      variant="flat"
      :loading="status === 'pending'"
      @click="pageData.cursor = data.next_cursor ?? ''"
    >
      加载更多
    </KunButton>

    <KunModal
      :model-value="isReasonModalOpen"
      inner-class-name="w-full max-w-lg"
      :is-dismissable="false"
      @update:model-value="closeReasonModal"
    >
      <div class="space-y-4">
        <h3 class="text-xl font-medium">{{ modalTitle }}</h3>
        <p class="text-default-500 text-sm">{{ modalDescription }}</p>

        <KunTextarea
          v-model="reasonText"
          :placeholder="
            modalRequiresReason ? '请填写拒绝原因 (必填)' : '可选填理由'
          "
          :rows="4"
          :maxlength="1007"
          show-char-count
          :required="modalRequiresReason"
        />

        <div class="flex justify-end gap-2">
          <KunButton variant="light" @click="closeReasonModal">取消</KunButton>
          <KunButton
            :color="reasonContext?.action === 'decline' ? 'danger' : 'default'"
            @click="handleConfirmReason"
          >
            确认
          </KunButton>
        </div>
      </div>
    </KunModal>
  </div>
</template>
