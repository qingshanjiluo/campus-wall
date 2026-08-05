<script setup lang="ts">
const props = defineProps<{
  ratingId?: number
  targetUserId: number
  likeCount: number
  isLiked: boolean
}>()

const { id } = usePersistUserStore()
const isLiked = ref(props.isLiked)
const likeCount = ref(props.likeCount)
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
  if (id === props.targetUserId) {
    useMessage(10236, 'warn')
    revert(next)
    return
  }
  pending.value = true
  const res = await kunFetch(`/galgame-rating/${props.ratingId}/like`, {
    method: 'PUT',
    body: { galgame_rating_id: props.ratingId }
  })
  pending.value = false
  if (!res) {
    revert(next)
    return
  }
  useMessage(next ? 10233 : 10234, 'success')
}
</script>

<template>
  <KunReaction
    v-model="isLiked"
    v-model:count="likeCount"
    :disabled="pending"
    icon="lucide:thumbs-up"
    color="primary"
    label="点赞"
    @change="onChange"
  />
</template>
