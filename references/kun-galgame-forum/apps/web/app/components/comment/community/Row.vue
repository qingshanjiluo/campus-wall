<script setup lang="ts">
// One post row in a community-primitive comment area — the SINGLE node component
// behind all six surfaces (galgame + rating / website / toolset / galgame-resource
// / quiz). Everything area-specific (endpoints, content cap, moderation key,
// anchor prefix, flat-vs-tree, mention support) comes from the target's descriptor
// in utils/communityComment.ts.
//
// The structural model is two tiers:
//
//   depth 0 = root; depth 1 = reply (any DB depth, rendered as one flat group)
//
// The container does the grouping and passes a root's replies down via `replies`;
// a reply node never receives children. Mutations bubble up as events; the
// container owns the list state.
//
// LAYOUT NOTES (the 2026-07 density pass). Three things carry the rhythm:
//   1. One action strip of uniform KunReaction pills, with every secondary action
//      behind ⋯. Previously the like pill sat beside taller icon buttons, so the
//      row never read as a single line.
//   2. An explicit meta → body → actions cadence (2 / 2.5) instead of a flat
//      space-y, so the body is the element with room around it.
//   3. Tier grouping is carried purely by SPACING CONTRAST — 4 inside a reply group
//      against 8 between roots (set by the containers). A rule + indent was tried
//      and deliberately dropped: the spacing alone separates the tiers, and the
//      extra chrome cost more than it bought.
const props = withDefaults(
  defineProps<{
    comment: GalgameCommunityComment
    target: CommunityCommentTarget
    depth?: number
    // Root-only: the flat reply group beneath this post (grouped by the
    // container). Always empty on a flat area and on a reply node.
    replies?: GalgameCommunityComment[]
  }>(),
  { depth: 0, replies: () => [] }
)

const emit = defineEmits<{
  replyAdded: [reply: GalgameCommunityComment]
  // Author edit success — the patched node, spliced in by id.
  updated: [post: GalgameCommunityComment]
  // Delete tombstones the post in place (the floor is kept); the container flips
  // its `deleted` flag rather than removing the row.
  tombstoned: [postId: number]
}>()

// A mounted section's target never changes kind, so the descriptor (and the
// permission check derived from it) is resolved once here rather than per render.
const surface = communityCommentSurface(props.target)

const { id } = usePersistUserStore()
const canDeleteComment = useCan(surface.deletePermission)
const { open: openFlag } = useGalgameCommentFlag()

const isShowReply = ref(false)
const isEditing = ref(false)
const editingContent = ref('')
const isSavingEdit = ref(false)

const isAuthor = computed(() => props.comment.user?.id === id)

// Authority is aligned with the server: only the author edits from the UI; the
// author OR a moderator deletes. A moderator CAN edit others server-side (the
// mod-actor variant), but no area ever offered that as a button, so it stays out.
const isShowEdit = computed(() => isAuthor.value && !props.comment.deleted)
const isShowDelete = computed(
  () => (isAuthor.value || canDeleteComment.value) && !props.comment.deleted
)
// Report entry: not on your own post, and never on a tombstone.
const isShowFlag = computed(() => !isAuthor.value && !props.comment.deleted)

// The ⋯ menu collects every secondary action (edit / report / delete), so the
// visible row stays three uniform pills. Hidden entirely when it would be empty —
// e.g. the author looking at their own tombstone.
const isShowMenu = computed(
  () => isShowEdit.value || isShowFlag.value || isShowDelete.value
)

// The "→ 对方" chip: only where the area actually wants it (never on galgame,
// which retired 「评论给」in favour of @mentions even though its replies still
// carry a server-completed target_user).
const replyTarget = computed(() =>
  surface.showsReplyTarget ? props.comment.target_user : null
)

// "已编辑" / "已编辑（管理）" — a moderator rewrite wins the label.
const editedLabel = computed(() => {
  if (props.comment.edited == null && !props.comment.edited_by_moderator) {
    return null
  }
  return props.comment.edited_by_moderator ? '已编辑（管理）' : '已编辑'
})

const handleFlag = () => {
  if (!id) {
    useAuthModal().open()
    return
  }
  openFlag(props.comment.id)
}

const handleStartEdit = () => {
  editingContent.value = props.comment.content
  isEditing.value = true
}

const handleCancelEdit = () => {
  isEditing.value = false
  editingContent.value = ''
}

const handleSubmitEdit = async () => {
  const text = editingContent.value.trim()
  if (!text) {
    useMessage(10540, 'warn')
    return
  }
  if (text.length > surface.maxLength) {
    useMessage(`评论最大长度为 ${surface.maxLength} 个字符`, 'warn')
    return
  }
  if (text === props.comment.content) {
    handleCancelEdit()
    return
  }

  isSavingEdit.value = true
  const updated = await kunFetch<GalgameCommunityComment>(
    surface.editUrl(props.comment.id),
    {
      method: 'PUT',
      query: surface.editQuery,
      body: { content: text }
    }
  )
  isSavingEdit.value = false

  if (updated) {
    useMessage('评论已更新', 'success')
    emit('updated', updated)
    handleCancelEdit()
  }
}

const handleDelete = async () => {
  const ok = await useComponentMessageStore().alert(
    '删除后此楼保留占位，回复不受影响，确定删除吗？'
  )
  if (!ok) {
    return
  }

  const result = await kunFetch(surface.deleteUrl(props.comment.id), {
    method: 'DELETE',
    query: surface.deleteQuery
  })

  if (result) {
    useMessage(10538, 'success')
    emit('tombstoned', props.comment.id)
  }
}

const handleReplyAdded = (reply: GalgameCommunityComment) => {
  isShowReply.value = false
  emit('replyAdded', reply)
}
</script>

<template>
  <!-- Anchor for notification / external deep-links and the post-publish scroll
       (?comment=<post_id> and #<prefix>-<post_id>); the container targets it. -->
  <div :id="`${surface.anchorPrefix}-${comment.id}`" class="flex gap-3">
    <KunAvatar :user="comment.user" :size="depth === 0 ? 'md' : 'sm'" />

    <div class="min-w-0 flex-1">
      <!-- Meta line. gap-x is wider than gap-y so a wrapped line still reads as
           one strip, and the name carries the only strong weight here. -->
      <div
        class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-xs leading-5"
      >
        <span class="text-default-800 text-sm font-medium">
          {{ comment.user.name }}
        </span>
        <template v-if="replyTarget">
          <KunIcon name="lucide:arrow-right" class="text-default-400" />
          <KunLink underline="hover" size="sm" :to="`/user/${replyTarget.id}`">
            {{ replyTarget.name }}
          </KunLink>
        </template>
        <span class="text-default-400">
          <KunTime :time="comment.created" />
        </span>
        <span v-if="editedLabel" class="text-default-400 italic">
          {{ editedLabel }}
        </span>
        <span
          v-if="comment.held"
          class="bg-warning-100 text-warning-600 rounded-full px-2 py-0.5"
        >
          审核中
        </span>
      </div>

      <!-- Tombstone: keep the floor, gray the body. -->
      <p v-if="comment.deleted" class="text-default-400 mt-2 text-sm italic">
        [已删除]
      </p>

      <!-- View mode: kungal-rendered Markdown (KunContent → DOMPurify). The BFF
           renders content_html for every area from the same pipeline. -->
      <KunContent
        v-else-if="!isEditing"
        class="mt-2"
        compact
        :content="renderKatex(comment.content_html)"
      />

      <!-- Edit mode: same editor as the composer, pre-loaded with the source. -->
      <div v-else class="mt-2 space-y-2">
        <KunMilkdownDualEditorProvider
          :value-markdown="editingContent"
          @set-markdown="(val) => (editingContent = val)"
        />
        <div class="flex justify-end gap-2">
          <KunButton
            variant="light"
            color="default"
            size="sm"
            @click="handleCancelEdit"
          >
            取消
          </KunButton>
          <KunButton
            size="sm"
            :loading="isSavingEdit"
            @click="handleSubmitEdit"
          >
            保存
          </KunButton>
        </div>
      </div>

      <!-- Action row: ONE strip of uniform KunReaction pills. Everything here is
           the same component in the same skin (`:toggle="false"` = one-shot action
           in the like pill's clothing), so the like button finally sits flush with
           its neighbours instead of next to taller icon buttons. Secondary actions
           live behind ⋯ — same shape as the topic reply footer, which is where this
           pattern is already established. -->
      <div
        v-if="!comment.deleted && !isEditing"
        class="mt-2.5 flex items-center gap-1"
      >
        <KunTooltip text="回复">
          <KunReaction
            :toggle="false"
            icon="lucide:reply"
            label="回复"
            @click="isShowReply = !isShowReply"
          />
        </KunTooltip>

        <CommentCommunityLike :comment="comment" />

        <KunPopover v-if="isShowMenu" position="bottom-start">
          <template #trigger>
            <KunReaction :toggle="false" icon="lucide:ellipsis" label="更多" />
          </template>

          <div class="flex w-44 flex-col gap-2 p-2">
            <KunButton
              v-if="isShowEdit"
              variant="light"
              color="default"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="handleStartEdit"
            >
              <KunIcon class-name="text-lg" name="lucide:pencil" />
              编辑评论
            </KunButton>

            <KunButton
              v-if="isShowFlag"
              variant="light"
              color="danger"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="handleFlag"
            >
              <KunIcon class-name="text-lg" name="lucide:flag" />
              举报评论
            </KunButton>

            <KunButton
              v-if="isShowDelete"
              variant="light"
              color="danger"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="handleDelete"
            >
              <KunIcon class-name="text-lg" name="lucide:trash-2" />
              删除评论
            </KunButton>
          </div>
        </KunPopover>
      </div>

      <KunFadeCard>
        <!-- A flat area has no parent pointer: replying there simply pre-targets
             THIS post's author and the new post lands flat in the list. -->
        <CommentCommunityComposer
          v-if="isShowReply"
          class="mt-3"
          :target="target"
          :reply-to-post-id="surface.isFlat ? null : comment.id"
          :target-user-id="comment.user.id"
          :is-reply="true"
          @close="isShowReply = false"
          @submitted="handleReplyAdded"
        />
      </KunFadeCard>

      <!-- Reply tier. Replies render flush — the smaller avatar plus the tighter
           spacing inside the group (4 here against 8 between roots) is what marks
           them; the grouping is carried by that spacing contrast rather than by any
           rule or indent. -->
      <div
        v-if="!surface.isFlat && depth === 0 && replies.length"
        class="mt-4 space-y-4"
      >
        <CommentCommunityRow
          v-for="reply in replies"
          :key="reply.id"
          :comment="reply"
          :target="target"
          :depth="1"
          @reply-added="(r) => emit('replyAdded', r)"
          @updated="(u) => emit('updated', u)"
          @tombstoned="(pid) => emit('tombstoned', pid)"
        />
      </div>
    </div>
  </div>
</template>
