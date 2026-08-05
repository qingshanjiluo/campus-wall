<script setup lang="ts">
const props = defineProps<{
  replyId: number
  targetUser: KunUser
  // Set when replying to another comment (nested); omitted for a top-level
  // comment posted on the reply itself.
  parentCommentId?: number
}>()

const emits = defineEmits<{
  getComment: [newComment: TopicComment]
  closePanel: []
}>()

const { name } = usePersistUserStore()
const topicId = inject<number>('topicId')
const commentValue = ref('')
const isPublishing = ref(false)

const handlePublishComment = async () => {
  if (!commentValue.value.trim()) {
    useMessage(10221, 'warn')
    return
  }

  if (commentValue.value.trim().length > 1007) {
    useMessage(10222, 'warn')
    return
  }

  isPublishing.value = true
  const comment = await kunFetch<TopicComment>(
    `/topic/${topicId}/comment`,
    {
      method: 'POST',
      body: {
        topic_id: topicId,
        reply_id: props.replyId,
        target_user_id: props.targetUser.id,
        parent_comment_id: props.parentCommentId,
        content: commentValue.value
      }
    }
  )
  isPublishing.value = false

  if (comment) {
    emits('getComment', comment)
    useMessage(10224, 'success')
    emits('closePanel')
  }
}

const handleClose = () => {
  emits('closePanel')
}
</script>

<template>
  <div class="w-full space-y-3 pt-2">
    <div class="flex items-center gap-1">
      {{ `${name} 评论` }}
      <KunUserChip size="sm" :user="targetUser" />
    </div>

    <KunTextarea
      name="comment"
      placeholder="请输入您的评论, 最大字数为1007"
      :rows="5"
      v-model="commentValue"
    />

    <div class="flex w-full justify-end space-x-1">
      <KunButton variant="light" color="danger" @click="handleClose">
        关闭
      </KunButton>
      <KunButton
        :disabled="isPublishing"
        :loading="isPublishing"
        @click="handlePublishComment"
      >
        发布评论
      </KunButton>
    </div>
  </div>
</template>
