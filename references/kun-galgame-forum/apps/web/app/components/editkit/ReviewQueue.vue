<script setup lang="ts">
// The review queue's list shell: status filter tabs + the proposal list with
// an empty state. Each row renders through the host's #item slot (falling
// back to a bare ProposalCard), so entity naming stays a host concern.
import { computed } from 'vue'
import type { KunTabItem } from '@kungal/ui-vue'
import type { EditProposal, EditUser } from './types'

const props = defineProps<{
  items: EditProposal[]
  users?: Record<number, EditUser>
  labelFor: (key: string) => string
  loading?: boolean
}>()

const status = defineModel<string>('status', { default: 'open' })

const statusTabs: KunTabItem[] = [
  { value: 'open', textValue: '待审核' },
  { value: 'merged', textValue: '已合并' },
  { value: 'declined', textValue: '已拒绝' },
  { value: 'withdrawn', textValue: '已撤回' }
]

const userOf = computed(() => (uid?: number) => {
  return uid !== undefined ? props.users?.[uid] : undefined
})
</script>

<template>
  <div class="space-y-3">
    <KunTab v-model="status" :items="statusTabs" variant="underlined" size="md" />

    <div v-if="loading" class="flex justify-center py-8">
      <KunLoading description="加载中…" />
    </div>

    <KunNull v-else-if="!items.length" description="这里空空如也" />

    <div v-else class="space-y-3">
      <template v-for="proposal in items" :key="proposal.id">
        <slot name="item" :proposal="proposal">
          <EditkitProposalCard
            :proposal="proposal"
            :label-for="labelFor"
            :proposer="userOf(proposal.proposer_uid)"
            :decider="userOf(proposal.decided_by_uid)"
          />
        </slot>
      </template>
    </div>
  </div>
</template>
