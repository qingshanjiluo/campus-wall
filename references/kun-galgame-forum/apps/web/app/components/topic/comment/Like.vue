<script setup lang="ts">
const props = defineProps<{
  comment: TopicComment
}>()

const { id } = usePersistUserStore()
const isLiked = ref(props.comment.is_liked)
const likeCount = ref(props.comment.like_count)
const pending = ref(false)

const revert = (next: boolean) => {
  isLiked.value = !next
  likeCount.value += next ? -1 : 1
}

const onChange = async (next: boolean) => {
  if (!id) {
    useAuthModal().open()
    revert(next)
    return
  }
  if (id === props.comment.user.id) {
    useMessage(10218, 'warn')
    revert(next)
    return
  }
  pending.value = true
  const result = await kunFetch<string>(
    `/topic/${props.comment.topic_id}/comment/like`,
    { method: 'PUT', body: { comment_id: props.comment.id } }
  )
  pending.value = false
  if (!result) {
    revert(next)
    return
  }
  useMessage(next ? '点赞评论成功' : '取消点赞成功', 'success')
}
</script>

<template>
  <KunReaction
    v-model="isLiked"
    v-model:count="likeCount"
    :disabled="pending"
    size="sm"
    icon="lucide:thumbs-up"
    color="primary"
    label="点赞"
    @change="onChange"
  />
</template>
