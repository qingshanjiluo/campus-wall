<script setup lang="ts">
const props = defineProps<{
  topicId?: number
  targetUserId: number
  upvoteCount: number
  isUpvoted: boolean
  // Render as a left-justified labeled row for the ⋯ overflow menu.
  menu?: boolean
}>()

const { id, moemoepoint } = usePersistUserStore()
const isUpvoted = ref(props.isUpvoted)
const upvoteCount = ref(props.upvoteCount)

// This component only TRIGGERS the one global 推话题 dialog (useUpvoteModal →
// TopicUpvoteModal at app root). It must not own the modal: in the mobile ⋯
// menu this row lives inside a KunPopover, and a modal mounted here dies with
// the popover the moment the user taps 确定推. See useUpvoteModal.
const { open } = useUpvoteModal()

// Both the menu button + the reaction icon funnel here. Repeatable — a topic can
// be pushed again and again — so there's NO "already upvoted" guard; every click
// is a fresh push (each costs 10 萌萌点 + credits the author 5).
const handleClickUpvote = async () => {
  if (!id) {
    useAuthModal().open()
    return
  }
  if (id === props.targetUserId) {
    useMessage(10241, 'warn')
    return
  }
  if (moemoepoint < 10) {
    useMessage(10242, 'warn')
    return
  }
  if (!props.topicId) {
    return
  }
  const pushed = await open({
    topicId: props.topicId,
    targetUserId: props.targetUserId
  })
  if (pushed) {
    upvoteCount.value++
    isUpvoted.value = true
  }
}
</script>

<template>
  <KunButton
    v-if="menu"
    :variant="isUpvoted ? 'flat' : 'light'"
    :color="isUpvoted ? 'secondary' : 'default'"
    size="sm"
    class-name="w-full justify-start gap-2 whitespace-nowrap"
    @click="handleClickUpvote"
  >
    <KunIcon class-name="text-lg" name="lucide:sparkles" />
    推话题
    <span v-if="upvoteCount" class="text-default-500 ml-auto">
      {{ upvoteCount }}
    </span>
  </KunButton>

  <KunTooltip v-else text="推话题">
    <!-- Action mode (repeatable): each click is a fresh push, never disabled
         after upvoting. The count rolls when a push lands. -->
    <KunReaction
      :toggle="false"
      :count="upvoteCount"
      label="推话题"
      @click="handleClickUpvote"
    >
      <template #icon>
        <KunIcon name="lucide:sparkles" class="text-warning" />
      </template>
    </KunReaction>
  </KunTooltip>
</template>
