<script setup lang="ts">
defineProps<{
  reply: SearchResultReply
}>()
</script>

<template>
  <KunLink
    color="default"
    underline="none"
    :to="replyPermalink(`/topic/${reply.topic_id}`, reply.floor)"
    class="flex-col items-start"
  >
    <div class="flex items-center gap-2">
      <KunIcon class="text-primary h-5 w-5" name="carbon:reply" />
      <span class="text-lg">{{ reply.topic_title }}</span>
      <span class="text-default-500 ml-auto text-sm">
        <KunTime :time="reply.created" />
      </span>
    </div>

    <div v-if="reply.content" class="my-2 rounded">
      <div class="mb-2 flex items-center">
        <KunAvatar :user="reply.user" :is-navigation="false" />
        <span class="ml-2 text-sm">{{ reply.user.name }}</span>
      </div>
      {{ markdownToText(reply.content) }}
    </div>
  </KunLink>
</template>
