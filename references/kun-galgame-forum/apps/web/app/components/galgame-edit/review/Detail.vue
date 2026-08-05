<script setup lang="ts">
// The review workbench (E3a's crown): per-field adjudication over one
// proposal. For every proposed field the reviewer sees current → proposed,
// and can (a) accept as-is, (b) CORRECT the value inline, or (c) reject just
// that field — the Google Docs suggestion-mode mental model. Corrections and
// rejections land as ONE amendment (double attribution on the merged
// revision); merge/decline close the proposal.
import { editValueEqual } from '~/components/editkit/utils'
import {
  galgameEditFieldConfig,
  galgameEditLabel
} from '~/constants/galgameEdit'

const route = useRoute()
const proposalId = computed(() => parseInt((route.params as { id: string }).id))

useKunDisableSeo('审阅提案')

// Entry authorization lives in the BFF (E3b: moderators AND the game's
// creator pass) — the page trusts the fetch outcome instead of duplicating
// the policy client-side. Only the exit destination branches: moderators go
// back to the site-wide queue, owners to their game's edit page. Proxy-face:
// this VIEW gate mirrors the infra editing-engine review capability (truth =
// infra, not pkg/perm), so it stays on useRole rather than useCan.
const { canModerate } = useRole()

const { data, status, refresh } = await useKunFetch<GalgameEditProposalDetail>(
  `/galgame-edit/proposals/${proposalId.value}`,
  { method: 'GET', watch: false }
)

// Adjudication right (amend / merge / decline) is a per-proposal projection from
// the engine: view (moderator+ or owner) is split from decide (admin+ or owner).
// A plain moderator without can_decide sees the proposal read-only. Proxy-face
// capability — sourced from the detail response, not pkg/perm.
const canDecide = computed(() => data.value?.can_decide ?? false)

const proposal = computed(() => data.value?.proposal)
const isOpen = computed(() => proposal.value?.status === 'open')
const exitTo = computed(() =>
  canModerate.value
    ? '/galgame-edit/review'
    : `/galgame/${proposal.value?.gid ?? ''}/edit`
)
const effective = computed(
  () => proposal.value?.effective_patch ?? proposal.value?.patch ?? {}
)
const fieldOf = (key: string) => data.value?.fields.find((f) => f.key === key)

// ---- per-field review state -------------------------------------------------
// overrides: reviewer-corrected values (amend set); rejected: field keys the
// reviewer strikes from the patch (amend unset). Editing state is local until
// 合并/保存修正 sends ONE amendment.
const overrides = reactive<Record<string, unknown>>({})
const editing = reactive<Record<string, boolean>>({})
const rejected = reactive<Record<string, boolean>>({})

const startEdit = (key: string) => {
  if (!(key in overrides)) {
    overrides[key] = structuredClone(toRaw(effective.value[key]) ?? null)
  }
  editing[key] = true
  rejected[key] = false
}

const cancelEdit = (key: string) => {
  Reflect.deleteProperty(overrides, key)
  editing[key] = false
}

const toggleReject = (key: string) => {
  rejected[key] = !rejected[key]
  if (rejected[key]) {
    editing[key] = false
    Reflect.deleteProperty(overrides, key)
  }
}

const amendSet = computed<Record<string, unknown>>(() => {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(overrides)) {
    if (editing[key] && !editValueEqual(value, effective.value[key])) {
      out[key] = value
    }
  }
  return out
})
const amendUnset = computed(() =>
  Object.keys(rejected).filter((key) => rejected[key])
)
const hasAmendment = computed(
  () => Object.keys(amendSet.value).length > 0 || amendUnset.value.length > 0
)
const remainingKeys = computed(() =>
  Object.keys(effective.value).filter((key) => !rejected[key])
)

const note = ref('')
const acting = ref(false)

// One amendment (when the reviewer changed anything), then merge.
const handleMerge = async () => {
  if (acting.value || !proposal.value) {
    return
  }
  if (!remainingKeys.value.length) {
    useMessage('所有字段都被拒绝了——请直接拒绝这个提案', 'warn')
    return
  }
  acting.value = true
  if (hasAmendment.value) {
    const amended = await kunFetch<unknown>(
      `/galgame-edit/proposals/${proposalId.value}/amend`,
      {
        method: 'POST',
        body: {
          set: amendSet.value,
          unset: amendUnset.value,
          note: note.value
        }
      }
    )
    if (!amended) {
      acting.value = false
      return
    }
  }
  const merged = await kunFetch<unknown>(
    `/galgame-edit/proposals/${proposalId.value}/merge`,
    { method: 'POST', body: { note: note.value } }
  )
  acting.value = false
  if (merged) {
    useMessage(
      hasAmendment.value ? '已修正并合并（双方署名）' : '提案已合并',
      'success'
    )
    await navigateTo(exitTo.value)
  }
}

const declineOpen = ref(false)
const handleDecline = async () => {
  if (acting.value || !note.value.trim()) {
    if (!note.value.trim()) {
      useMessage('请先在下方填写拒绝理由', 'warn')
    }
    return
  }
  acting.value = true
  const declined = await kunFetch<unknown>(
    `/galgame-edit/proposals/${proposalId.value}/decline`,
    { method: 'POST', body: { note: note.value } }
  )
  acting.value = false
  declineOpen.value = false
  if (declined) {
    useMessage('提案已拒绝', 'success')
    await navigateTo(exitTo.value)
  }
}

const userName = (uid?: number) => {
  if (uid === undefined) {
    return ''
  }
  return data.value?.users?.[uid]?.name ?? `用户 #${uid}`
}
</script>

<template>
  <div class="mx-auto flex max-w-3xl flex-col gap-3">
    <template v-if="data && proposal">
      <KunCard
        :is-hoverable="false"
        :is-transparent="false"
        content-class="space-y-2"
      >
        <KunHeader :name="`审阅提案 #${proposal.id}`" scale="h2" />
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <KunLink :to="`/galgame/${proposal.gid}`" size="sm">
            前往条目 #{{ proposal.gid }}
          </KunLink>
          <span class="text-default-400">
            提案人：{{ userName(proposal.proposer_uid) }} ·
            <KunTime :time="proposal.created_at" type="date" show-year />
          </span>
          <KunButton
            variant="light"
            color="default"
            size="sm"
            class-name="ml-auto"
            @click="navigateTo(exitTo)"
          >
            <KunIcon name="lucide:arrow-left" />
            {{ canModerate ? '返回队列' : '返回编辑页' }}
          </KunButton>
        </div>
        <KunInfo
          v-if="proposal.note"
          color="info"
          title="提案说明"
          :description="proposal.note"
        />
        <KunInfo
          v-if="proposal.status === 'declined' && proposal.decision_note"
          color="danger"
          title="拒绝理由"
          :description="proposal.decision_note"
        />
      </KunCard>

      <!-- Amendment trail: every past correction, permanently attributed -->
      <KunCard
        v-if="proposal.amendments?.length"
        :is-hoverable="false"
        :is-transparent="false"
        content-class="space-y-2"
      >
        <KunHeader name="审核修正记录" scale="h3" />
        <div
          v-for="a in proposal.amendments"
          :key="a.id"
          class="border-default-200 rounded border p-2 text-sm"
        >
          <p class="text-default-500">
            #{{ a.seq }} · {{ userName(a.amender_uid) }}
            <KunTime :time="a.created_at" type="date" show-year />
          </p>
          <div class="mt-1 flex flex-wrap gap-1">
            <KunChip
              v-for="key in Object.keys(a.set ?? {})"
              :key="`set-${key}`"
              size="sm"
              variant="flat"
              color="secondary"
            >
              修正 {{ galgameEditLabel(key) }}
            </KunChip>
            <KunChip
              v-for="key in a.unset ?? []"
              :key="`unset-${key}`"
              size="sm"
              variant="flat"
              color="danger"
            >
              拒绝 {{ galgameEditLabel(key) }}
            </KunChip>
          </div>
          <p v-if="a.note" class="text-default-400 mt-1 text-xs">
            {{ a.note }}
          </p>
        </div>
      </KunCard>

      <!-- Per-field adjudication -->
      <KunCard
        :is-hoverable="false"
        :is-transparent="false"
        content-class="space-y-5"
      >
        <KunHeader
          name="逐字段审阅"
          :description="
            isOpen
              ? '每个字段可原样接受、修正后接受、或单独拒绝——修正后合并将同时署名提案人与审核人'
              : '提案已关闭，以下为最终内容'
          "
          scale="h3"
        />

        <div
          v-for="(value, key) in effective"
          :key="key"
          class="space-y-2"
          :class="rejected[key] ? 'opacity-50' : ''"
        >
          <EditkitFieldDiff
            :label="galgameEditLabel(String(key))"
            :diff-hint="fieldOf(String(key))?.diff_hint"
            :from="data.values[String(key)]"
            :to="editing[String(key)] ? overrides[String(key)] : value"
            :config="galgameEditFieldConfig(String(key))"
          />

          <div
            v-if="isOpen && canDecide && fieldOf(String(key))?.can_review"
            class="flex flex-wrap items-center gap-2"
          >
            <template v-if="!editing[String(key)]">
              <KunButton
                variant="flat"
                color="secondary"
                size="sm"
                :disabled="rejected[String(key)]"
                @click="startEdit(String(key))"
              >
                <KunIcon name="lucide:pencil" />
                修正该值
              </KunButton>
            </template>
            <template v-else>
              <KunButton
                variant="flat"
                color="default"
                size="sm"
                @click="cancelEdit(String(key))"
              >
                取消修正
              </KunButton>
            </template>
            <KunButton
              variant="flat"
              :color="rejected[String(key)] ? 'default' : 'danger'"
              size="sm"
              @click="toggleReject(String(key))"
            >
              <KunIcon name="lucide:x" />
              {{ rejected[String(key)] ? '恢复该字段' : '拒绝该字段' }}
            </KunButton>
          </div>

          <!-- Inline correction editor (the crown interaction) -->
          <div
            v-if="
              isOpen &&
              canDecide &&
              editing[String(key)] &&
              fieldOf(String(key))
            "
            class="border-secondary-200 rounded border p-3"
          >
            <EditkitSchemaField
              v-model="overrides[String(key)]"
              :field="fieldOf(String(key))!"
              :config="galgameEditFieldConfig(String(key))"
            />
          </div>
        </div>

        <KunNull
          v-if="!Object.keys(effective).length"
          description="提案的所有字段都已被移除"
        />
      </KunCard>

      <!-- View-only state: a moderator may open the proposal but only an
           adjudicator (admin+ or the game's owner) can amend / merge / decline. -->
      <KunInfo
        v-if="isOpen && !canDecide"
        color="info"
        title="只读审阅"
        description="您可以查看此提案，但只有具备裁决权限的管理员（或该条目的创建者）可以合并、修正或拒绝。"
      />

      <!-- Decision bar -->
      <KunCard
        v-if="isOpen && canDecide"
        :is-hoverable="false"
        :is-transparent="false"
        content-class="space-y-3"
      >
        <KunTextarea
          v-model="note"
          label="审核说明"
          placeholder="合并备注，或拒绝理由（拒绝时必填）"
          :maxlength="2000"
        />
        <div class="flex flex-wrap items-center justify-end gap-2">
          <span v-if="hasAmendment" class="text-secondary-600 text-sm">
            将先保存 {{ Object.keys(amendSet).length + amendUnset.length }}
            处修正，再合并
          </span>
          <KunButton
            variant="flat"
            color="danger"
            :loading="acting"
            @click="declineOpen = true"
          >
            拒绝提案
          </KunButton>
          <KunButton color="primary" :loading="acting" @click="handleMerge">
            {{ hasAmendment ? '修正并合并' : '合并提案' }}
          </KunButton>
        </div>
      </KunCard>

      <KunModal v-model="declineOpen">
        <div class="space-y-3">
          <KunHeader name="拒绝这个提案？" scale="h3" />
          <p class="text-default-500 text-sm">
            拒绝理由将展示给提案人：{{
              note || '（尚未填写，请返回填写审核说明）'
            }}
          </p>
          <div class="flex justify-end gap-2">
            <KunButton
              variant="flat"
              color="default"
              @click="declineOpen = false"
            >
              取消
            </KunButton>
            <KunButton color="danger" :loading="acting" @click="handleDecline">
              确认拒绝
            </KunButton>
          </div>
        </div>
      </KunModal>
    </template>

    <KunNull
      v-else-if="status !== 'pending'"
      description="提案不存在或编辑服务暂不可用"
    />
  </div>
</template>
