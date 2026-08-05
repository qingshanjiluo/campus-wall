<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'

const props = defineProps<{
  replyId: number
  commentsData: TopicComment[]
}>()

const currentUserId = usePersistUserStore().id
const comments = ref(props.commentsData)
const activeCommentId = ref<number | null>(null)
const targetUserForPanel = ref<KunUser | null>(null)
// The comment being replied to (its id becomes the new comment's parent), so
// the reply nests under the right thread.
const parentCommentIdForPanel = ref<number | null>(null)

// Flatten the comment tree to a SINGLE visible level: each top-level comment
// followed by all its descendants (replies), ordered by time. The data model
// stores the full tree (parentCommentId) — future-proof — but rendering it one
// level deep reads best (especially on mobile); a reply shows "回复 @<target>".
const threadedComments = computed(() => {
  const list = comments.value
  const byId = new Map(list.map((c) => [c.id, c]))
  // A comment's top-level ancestor (walk up parentCommentId). A missing parent
  // — e.g. one that was deleted and re-rooted — makes the comment a root itself.
  const rootOf = (c: TopicComment): TopicComment => {
    let cur = c
    const seen = new Set<number>()
    while (
      cur.parent_comment_id != null &&
      byId.has(cur.parent_comment_id) &&
      !seen.has(cur.id)
    ) {
      seen.add(cur.id)
      cur = byId.get(cur.parent_comment_id)!
    }
    return cur
  }
  const byTime = (a: TopicComment, b: TopicComment) =>
    new Date(a.created).getTime() - new Date(b.created).getTime()

  const roots: TopicComment[] = []
  const childrenOf = new Map<number, TopicComment[]>()
  for (const c of list) {
    const root = rootOf(c)
    if (root.id === c.id) {
      roots.push(c)
    } else {
      const arr = childrenOf.get(root.id) ?? []
      arr.push(c)
      childrenOf.set(root.id, arr)
    }
  }
  roots.sort(byTime)

  const out: { comment: TopicComment; depth: number }[] = []
  for (const root of roots) {
    out.push({ comment: root, depth: 0 })
    for (const kid of (childrenOf.get(root.id) ?? []).slice().sort(byTime)) {
      out.push({ comment: kid, depth: 1 })
    }
  }
  return out
})

// Inline edit state (mirrors the reply "重新编辑" feature). Only the author
// can edit; saving PUTs to /topic/:tid/comment and replaces the comment in
// place with the server's updated DTO (which carries the new `edited` stamp).
const editingId = ref<number | null>(null)
const editValue = ref('')
const isSaving = ref(false)

// Mobile shows the comment editor in a bottom KunDrawer (one shared instance
// driven by activeCommentId); desktop keeps the inline per-comment panel. Gate
// behind mount so the first client render matches SSR (desktop) — useMediaQuery
// resolves true synchronously on a mobile client and would otherwise mismatch.
const isMobileQuery = useMediaQuery('(max-width: 767px)')
const mounted = ref(false)
onMounted(() => (mounted.value = true))
const isMobile = computed(() => mounted.value && isMobileQuery.value)
const isCommentPanelOpen = computed({
  get: () => activeCommentId.value !== null && !!targetUserForPanel.value,
  set: (open) => {
    if (!open) {
      activeCommentId.value = null
      targetUserForPanel.value = null
      parentCommentIdForPanel.value = null
    }
  }
})

const handleClickComment = (comment: TopicComment) => {
  if (!currentUserId) {
    useAuthModal().open()
    return
  }

  if (activeCommentId.value === comment.id) {
    activeCommentId.value = null
    targetUserForPanel.value = null
    parentCommentIdForPanel.value = null
  } else {
    activeCommentId.value = comment.id
    targetUserForPanel.value = comment.user
    parentCommentIdForPanel.value = comment.id
  }
}

const handleNewComment = (newComment: TopicComment) => {
  comments.value.push(newComment)
  activeCommentId.value = null
  targetUserForPanel.value = null
  parentCommentIdForPanel.value = null
}

const handleRemoveComment = (commentId: number) => {
  const index = comments.value.findIndex((c) => c.id === commentId)
  if (index !== -1) {
    comments.value.splice(index, 1)
  }
}

const handleStartEdit = (comment: TopicComment) => {
  editingId.value = comment.id
  editValue.value = comment.content
}

const handleCancelEdit = () => {
  editingId.value = null
  editValue.value = ''
}

const handleSaveEdit = async (comment: TopicComment) => {
  const content = editValue.value.trim()
  if (!content) {
    useMessage(10221, 'warn')
    return
  }
  if (content.length > 1007) {
    useMessage(10222, 'warn')
    return
  }

  isSaving.value = true
  const updated = await kunFetch<TopicComment>(
    `/topic/${comment.topic_id}/comment`,
    {
      method: 'PUT',
      body: { comment_id: comment.id, content }
    }
  )
  isSaving.value = false

  if (updated) {
    const index = comments.value.findIndex((c) => c.id === comment.id)
    if (index !== -1) {
      comments.value[index] = updated
    }
    editingId.value = null
    useMessage('编辑评论成功', 'success')
  }
}
</script>

<template>
  <div v-if="comments.length" class="bg-default-100 space-y-3 rounded-lg p-3">
    <h3 class="text-lg font-semibold">评论</h3>

    <div class="space-y-3">
      <!-- Threaded: depth-0 = top-level, depth-1 = a reply (indented one level
           under its thread root, no connector line). -->
      <div
        v-for="{ comment, depth } in threadedComments"
        :id="`comment-${comment.id}`"
        :key="comment.id"
        :class="depth === 1 ? 'ml-9' : ''"
      >
        <div class="flex items-start space-x-3">
          <KunAvatar :user="comment.user" />

          <div class="flex w-full flex-col space-y-1">
            <div class="flex items-center justify-between">
              <div class="text-sm">
                <span>{{ comment.user.name }}</span>
                <span class="text-default-500 mx-1">
                  {{ depth === 1 ? '回复' : '评论' }}
                </span>
                <KunLink
                  size="sm"
                  underline="hover"
                  :to="`/user/${comment.target_user.id}`"
                >
                  {{ comment.target_user.name }}
                </KunLink>
              </div>

              <div class="flex items-center gap-1">
                <KunButton
                  v-if="
                    comment.user.id === currentUserId &&
                    editingId !== comment.id
                  "
                  :is-icon-only="true"
                  variant="light"
                  color="default"
                  @click="handleStartEdit(comment)"
                >
                  <KunIcon name="lucide:pencil" />
                </KunButton>
                <TopicCommentDelete
                  @remove-comment="handleRemoveComment"
                  :comment="comment"
                />
              </div>
            </div>

            <!-- Edit mode: textarea + save / cancel -->
            <div v-if="editingId === comment.id" class="space-y-2">
              <KunTextarea
                name="edit-comment"
                placeholder="请输入您的评论, 最大字数为 1007"
                :rows="4"
                v-model="editValue"
              />
              <div class="flex justify-end gap-1">
                <KunButton
                  variant="light"
                  color="danger"
                  @click="handleCancelEdit"
                >
                  取消
                </KunButton>
                <KunButton
                  :disabled="isSaving"
                  :loading="isSaving"
                  @click="handleSaveEdit(comment)"
                >
                  保存
                </KunButton>
              </div>
            </div>

            <!-- View mode -->
            <p
              v-else
              style="overflow-wrap: break-word"
              class="text-default-700 text-sm whitespace-pre-wrap"
            >
              {{ comment.content }}
            </p>

            <div class="flex items-center justify-between">
              <span class="text-default-500 text-xs">
                <KunTime :time="comment.created" type="datetime" show-year />
                <span v-if="comment.edited" class="ml-1">
                  (编辑于
                  <KunTime :time="comment.edited" type="datetime" show-year />)
                </span>
              </span>

              <div class="flex gap-2">
                <TopicCommentLike :comment="comment" />
                <KunButton
                  :is-icon-only="true"
                  variant="light"
                  color="default"
                  class-name="gap-1"
                  @click="handleClickComment(comment)"
                >
                  <KunIcon name="uil:comment-dots" />
                </KunButton>
                <ReportButton
                  v-if="comment.user.id !== currentUserId"
                  subject-kind="forum_comment"
                  :subject-id="comment.id"
                  :snapshot="comment.content"
                  :subject-url="`${kungal.domain.main}/topic/${comment.topic_id}?comment=${comment.id}`"
                />
              </div>
            </div>
          </div>
        </div>

        <KunFadeCard v-if="!isMobile">
          <LazyTopicCommentPanel
            v-if="activeCommentId === comment.id && targetUserForPanel"
            :reply-id="replyId"
            :target-user="targetUserForPanel"
            :parent-comment-id="parentCommentIdForPanel ?? undefined"
            @get-comment="handleNewComment"
            @close-panel="activeCommentId = null"
          />
        </KunFadeCard>
      </div>
    </div>

    <KunDrawer
      v-if="isMobile"
      v-model="isCommentPanelOpen"
      placement="bottom"
      size="md"
      title="发表评论"
    >
      <LazyTopicCommentPanel
        v-if="targetUserForPanel"
        :reply-id="replyId"
        :target-user="targetUserForPanel"
        :parent-comment-id="parentCommentIdForPanel ?? undefined"
        @get-comment="handleNewComment"
        @close-panel="activeCommentId = null"
      />
    </KunDrawer>
  </div>
</template>
