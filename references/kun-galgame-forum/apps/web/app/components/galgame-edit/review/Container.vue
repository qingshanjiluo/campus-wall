<script setup lang="ts">
// The kungal review queue (E3a, moderator entry): open galgame.game
// proposals on the kungal tenant, enriched with game briefs. Field-level
// adjudication rights come from the engine's projection on the detail page —
// this list is the entry.
import { galgameEditLabel } from '~/constants/galgameEdit'

// Proxy-face gate: the review-queue entry mirrors the infra editing-engine
// review capability (truth = infra, not pkg/perm), so it stays on useRole
// rather than useCan. Per-proposal adjudication rights come from the engine
// projection on the detail page (can_decide).
const { canModerate } = useRole()

useKunDisableSeo('Galgame 提案审核队列')

const status = ref('open')

const { data, status: fetchStatus } =
  await useKunFetch<GalgameEditProposalList>('/galgame-edit/queue', {
    method: 'GET',
    query: { status }
  })

const briefName = (item: GalgameEditProposalItem): string => {
  if (!item.galgame) {
    return `Galgame #${item.gid || item.entity_id}`
  }
  const name: KunLanguage = {
    'en-us': item.galgame.name_en_us,
    'ja-jp': item.galgame.name_ja_jp,
    'zh-cn': item.galgame.name_zh_cn,
    'zh-tw': item.galgame.name_zh_tw
  }
  return getPreferredLanguageText(name) || `Galgame #${item.gid || item.entity_id}`
}
</script>

<template>
  <div class="mx-auto flex max-w-3xl flex-col gap-3">
    <KunCard :is-hoverable="false" :is-transparent="false">
      <KunHeader
        name="Galgame 提案审核队列"
        description="社区提交的资料修改提案。审核时可直接修正提案内容再合并（Google Docs 建议模式的心智）——提案人与审核人都会署名"
        scale="h2"
      />
    </KunCard>

    <KunNull v-if="!canModerate" description="需要管理权限" />

    <KunCard
      v-else
      :is-hoverable="false"
      :is-transparent="false"
      content-class="space-y-3"
    >
      <EditkitReviewQueue
        v-model:status="status"
        :items="data?.items ?? []"
        :users="data?.users"
        :label-for="galgameEditLabel"
        :loading="fetchStatus === 'pending'"
      >
        <template #item="{ proposal }">
          <EditkitProposalCard
            :proposal="proposal"
            :label-for="galgameEditLabel"
            :proposer="data?.users?.[proposal.proposer_uid]"
            :decider="
              proposal.decided_by_uid !== undefined
                ? data?.users?.[proposal.decided_by_uid]
                : undefined
            "
          >
            <template #title>
              <KunLink
                :to="`/galgame/${(proposal as GalgameEditProposalItem).gid}`"
                size="sm"
                class-name="font-medium"
              >
                {{ briefName(proposal as GalgameEditProposalItem) }}
              </KunLink>
            </template>
            <template #actions>
              <KunButton
                variant="flat"
                color="primary"
                size="sm"
                @click="navigateTo(`/galgame-edit/review/${proposal.id}`)"
              >
                {{ proposal.status === 'open' ? '审阅' : '查看' }}
              </KunButton>
            </template>
          </EditkitProposalCard>
        </template>
      </EditkitReviewQueue>
    </KunCard>
  </div>
</template>
