<script setup lang="ts">
// Community-primitive comment section for a QUIZ — rendered from
// pages/galgame-quiz/[id].vue.
//
// Spoiler gate: a quiz that conceals something (hidden linked galgames, or a
// non-trivial spoiler level) withholds its discussion from a viewer who has
// neither answered nor authored it — otherwise one "答案是 XX" top post retires the
// question. The ruling is made SERVER-side and arrives as `locked` on the page
// payload (api service/resource_comment_gate.go); this component only renders it,
// so the rule cannot drift between the two sides and cannot be bypassed by reading
// the network response directly.
const props = defineProps<{
  quizId: number
}>()

const {
  status,
  seeded,
  locked,
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
} = await useCommunityCommentList({ kind: 'quiz', quizId: props.quizId })

const target: CommunityCommentTarget = { kind: 'quiz', quizId: props.quizId }

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
      name="题目讨论"
      description="聊聊这道题目, 请不要直接剧透答案"
      scale="h2"
    >
      <template v-if="total > 0 && !locked" #endContent>
        <span class="text-default-500 text-sm">{{ total }} 条讨论</span>
      </template>
    </KunHeader>

    <KunLoading v-if="status === 'pending' && !seeded" />

    <!-- Gated: the server withheld the thread from this viewer. No composer
         either — a create is 403'd server-side for the same reason. -->
    <KunNull
      v-else-if="locked"
      description="这道题目含有剧透, 作答后即可查看并参与讨论"
    />

    <div v-else class="space-y-5">
      <CommentCommunityComposer :target="target" @submitted="onPublished" />

      <KunNull v-if="isEmpty" description="还没有人讨论这道题目" />

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
        加载更多讨论
      </KunButton>
    </div>

    <!-- Single community-comment flag modal for this section (region agnostic). -->
    <CommentCommunityFlagModal />
  </KunCard>
</template>
