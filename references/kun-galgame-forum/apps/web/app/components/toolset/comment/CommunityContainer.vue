<script setup lang="ts">
// Community-primitive toolset comment section (charter step 08) — the
// unconditional toolset comment UI, rendered from ToolsetDetail.vue (the legacy
// comment components were retired in charter step 06a).
//
// Paging / grouping / optimistic mutation come from useCommunityCommentList; the
// toolset owner's delete authority is a server-side superset (charter ruling 20),
// so this container needs no owner id — the UI shows delete on author||canModerate.
const props = defineProps<{
  toolsetId: number
}>()

const {
  status,
  seeded,
  groups,
  isEmpty,
  hasMore,
  loadingMore,
  loadMore,
  handleNewComment,
  handleUpdated,
  handleTombstoned
} = await useCommunityCommentList({
  kind: 'toolset',
  toolsetId: props.toolsetId
})

const target: CommunityCommentTarget = {
  kind: 'toolset',
  toolsetId: props.toolsetId
}
</script>

<template>
  <div class="space-y-5">
    <KunHeader
      name="评论"
      description="如果您对该工具有任何的使用疑问, 欢迎发布评论"
      scale="h2"
    />

    <CommentCommunityComposer :target="target" @submitted="handleNewComment" />

    <KunLoading v-if="status === 'pending' && !seeded" />

    <KunNull v-else-if="isEmpty" />

    <div v-else-if="groups.length" class="space-y-8">
      <CommentCommunityRow
        v-for="group in groups"
        :key="group.root.id"
        :comment="group.root"
        :replies="group.replies"
        :target="target"
        :depth="0"
        @reply-added="handleNewComment"
        @updated="handleUpdated"
        @tombstoned="handleTombstoned"
      />
    </div>

    <KunButton
      v-if="hasMore"
      variant="light"
      color="primary"
      full-width
      :loading="loadingMore"
      @click="loadMore"
    >
      <KunIcon name="lucide:chevron-down" />
      加载更多评论
    </KunButton>

    <!-- Single community-comment flag modal (reused, region agnostic). -->
    <CommentCommunityFlagModal />
  </div>
</template>
