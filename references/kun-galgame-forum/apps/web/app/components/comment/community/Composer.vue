<script setup lang="ts">
// Compose-box for a community-primitive comment — the SINGLE composer behind all
// four surfaces. Authoring is the galgame area's KunMilkdownDualEditorProvider
// everywhere (the wire payload is raw Markdown, which the BFF re-renders into
// content_html through kungal's own pipeline for every area).
//
//   - Root mode: no replyToPostId. The container renders it above the thread.
//   - Reply mode: replyToPostId set. CommentCommunityRow renders it inline beneath
//     the post being replied to (the server derives root/target pointers).
//
// A FLAT area (rating) has no parent pointer: it passes replyToPostId=null and an
// explicit targetUserId instead (charter ruling 19 — the create rejects a missing
// recipient there).
const props = withDefaults(
  defineProps<{
    target: CommunityCommentTarget
    replyToPostId?: number | null
    // Required on a flat area: the recipient. Top-level = the resource author;
    // reply = the replied post's author.
    targetUserId?: number
    // Reply mode drives the button label only. It is declared separately from
    // replyToPostId because a FLAT area replies with a null parent pointer — the
    // label cannot be inferred from the payload there.
    isReply?: boolean
  }>(),
  { replyToPostId: null, targetUserId: undefined, isReply: false }
)

const emits = defineEmits<{
  close: []
  // Optimistic update: the server-returned post so the parent can splice it into
  // the list without re-fetching. A held (TL0) post still comes back so its
  // author sees their own "审核中" comment.
  submitted: [post: GalgameCommunityComment]
}>()

// The target never changes kind while mounted — resolve the descriptor once.
const surface = communityCommentSurface(props.target)

const { id } = usePersistUserStore()

const content = ref('')
const isPublishing = ref(false)

const handlePublish = async () => {
  if (!id) {
    useAuthModal().open()
    return
  }
  const trimmed = content.value.trim()
  if (!trimmed) {
    useMessage(10540, 'warn')
    return
  }
  if (trimmed.length > surface.maxLength) {
    useMessage(`评论最大长度为 ${surface.maxLength} 个字符`, 'warn')
    return
  }

  isPublishing.value = true
  const result = await kunFetch<GalgameCommunityComment>(surface.listUrl, {
    method: 'POST',
    query: surface.addressQuery,
    body: surface.createBody(trimmed, props.replyToPostId, props.targetUserId)
  })
  isPublishing.value = false

  if (result) {
    content.value = ''
    // A TL0 first post is held for review; the receipt says so, but we still
    // insert it locally (the author sees their own held post).
    useMessage(result.held ? '已提交，待审核' : 10542, 'success')
    emits('submitted', result)
    emits('close')
  }
}
</script>

<template>
  <div class="space-y-3">
    <KunMilkdownDualEditorProvider
      :value-markdown="content"
      :placeholder="surface.composerPlaceholder"
      @set-markdown="(val) => (content = val)"
    />

    <div class="flex items-center justify-between gap-2">
      <slot />

      <KunButton class="ml-auto" :loading="isPublishing" @click="handlePublish">
        {{ isReply ? '发布回复' : '发布评论' }}
      </KunButton>
    </div>
  </div>
</template>
