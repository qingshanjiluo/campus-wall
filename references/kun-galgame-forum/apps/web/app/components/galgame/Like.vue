<script setup lang="ts">
const props = defineProps<{
  galgameId: number
  targetUserId: number
  likeCount: number
  isLiked: boolean
}>()

const { id } = usePersistUserStore()
const isLiked = ref(props.isLiked)
const likesCount = ref(props.likeCount)

// The feed hydrates is-liked asynchronously (see useMyGalgameInteractions), so
// reflect a late-arriving initial state. Harmless on the detail page where the
// prop is already settled.
watch(
  () => props.isLiked,
  (value) => (isLiked.value = value)
)

const pending = ref(false)
const revert = (next: boolean) => {
  isLiked.value = !next
  likesCount.value += next ? -1 : 1
}

const onChange = async (next: boolean) => {
  if (!id) {
    useAuthModal().open()
    revert(next)
    return
  }
  if (id === props.targetUserId) {
    useMessage(10533, 'warn')
    revert(next)
    return
  }
  pending.value = true
  const result = await kunFetch(`/galgame/${props.galgameId}/like`, {
    method: 'PUT'
  })
  pending.value = false
  if (!result) {
    revert(next)
    return
  }
  useMessage(next ? 10530 : 10531, 'success')
}
</script>

<template>
  <KunTooltip text="点赞">
    <!-- The span is load-bearing: KunTooltip wraps its child in an inline-block,
         whose line-height leading pushes the pill ~1.6px above its neighbours in
         the header's `items-center` row. A flex wrapper has no line box, so the
         pill sits flush — 收藏 already does this, and 点赞 was the odd one out. -->
    <span class="flex">
      <KunReaction
        v-model="isLiked"
        v-model:count="likesCount"
        :disabled="pending"
        icon="lucide:thumbs-up"
        color="primary"
        label="点赞"
        @change="onChange"
      />
    </span>
  </KunTooltip>
</template>
