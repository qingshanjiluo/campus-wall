<script setup lang="ts">
const props = defineProps<{
  initialReplies: TopicReply[]
  topicId: number
  title: string
}>()

const replies = ref<TopicReply[]>([])
watch(
  () => props.initialReplies,
  (newVal) => {
    replies.value = JSON.parse(JSON.stringify(newVal))
  },
  { immediate: true, deep: true }
)
</script>

<template>
  <div class="flex flex-col gap-4">
    <TopicReply
      v-for="reply in replies"
      :key="reply.id"
      :reply="reply"
      :title="title"
    />
  </div>
</template>
