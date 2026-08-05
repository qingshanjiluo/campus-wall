<script setup lang="ts">
// Community-primitive rating comment section (charter step 08) — the
// unconditional rating comment UI, rendered from GalgameRatingDetailDetail.vue
// (the legacy comment components were retired in charter step 06a).
//
// The rating area is FLAT: every post is a root carrying an explicit target_user
// ("A → B") rather than a parent pointer. `isFlat` on the descriptor is what makes
// the shared grouping degrade to one-group-per-post, and what makes a reply pass a
// recipient instead of a parent id — so this container shares the same
// useCommunityCommentList machinery as the tree areas.
const props = defineProps<{
  ratingId: number
  ratingAuthor: KunUser
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
} = await useCommunityCommentList({ kind: 'rating', ratingId: props.ratingId })

const target: CommunityCommentTarget = {
  kind: 'rating',
  ratingId: props.ratingId
}
</script>

<template>
  <KunCard :is-transparent="false" :is-hoverable="false">
    <KunHeader
      name="评论区"
      description="发布对这个评分的观点, 请不要锐评"
      scale="h2"
    />

    <div class="space-y-5">
      <!-- Top-level comments on a rating are addressed to its author. -->
      <CommentCommunityComposer
        :target="target"
        :target-user-id="ratingAuthor.id"
        @submitted="handleNewComment"
      />

      <KunLoading v-if="status === 'pending' && !seeded" />

      <KunNull v-else-if="isEmpty" />

      <div v-else-if="groups.length" class="space-y-8">
        <CommentCommunityRow
          v-for="group in groups"
          :key="group.root.id"
          :comment="group.root"
          :target="target"
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

    <!-- Single community-comment flag modal for this section (reused, region
         agnostic — post-id addressed). -->
    <CommentCommunityFlagModal />
  </KunCard>
</template>
