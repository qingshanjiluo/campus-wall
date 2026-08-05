<script setup lang="ts">
// Community-primitive comment section for a galgame RESOURCE — rendered from
// pages/galgame-resource/[id]/index.vue.
//
// A new area introduced directly on the primitive (no frozen local table behind
// it), so it has no legacy parity to preserve: it renders through the shared
// comment family and gets its paging from useCommunityCommentList. The resource's
// uploader is notified of top-level comments and may delete any comment here (both
// server-side), which is why this container needs no owner prop.
const props = defineProps<{
  resourceId: number
}>()

const {
  status,
  seeded,
  groups,
  isEmpty,
  total,
  hasMore,
  loadingMore,
  loadMore,
  handleNewComment,
  handleUpdated,
  handleTombstoned,
  scrollToPost
} = await useCommunityCommentList({
  kind: 'resource',
  resourceId: props.resourceId
})

const target: CommunityCommentTarget = {
  kind: 'resource',
  resourceId: props.resourceId
}

// Scroll to a freshly published root so the author sees their own post land.
const onPublished = (post: GalgameCommunityComment) => {
  handleNewComment(post)
  if (post.root_comment_id == null) {
    scrollToPost(post.id)
  }
}
</script>

<template>
  <KunCard :is-transparent="false" :is-hoverable="false">
    <KunHeader
      name="资源评论"
      description="这个资源能正常使用吗? 有问题欢迎在这里反馈"
      scale="h2"
    >
      <template v-if="total > 0" #endContent>
        <span class="text-default-500 text-sm">{{ total }} 条评论</span>
      </template>
    </KunHeader>

    <div class="space-y-5">
      <CommentCommunityComposer :target="target" @submitted="onPublished" />

      <KunLoading v-if="status === 'pending' && !seeded" />

      <KunNull v-else-if="isEmpty" description="还没有人评论这个资源" />

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
    </div>

    <!-- Single community-comment flag modal for this section (region agnostic). -->
    <CommentCommunityFlagModal />
  </KunCard>
</template>
