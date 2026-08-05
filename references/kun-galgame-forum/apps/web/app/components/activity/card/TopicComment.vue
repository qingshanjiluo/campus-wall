<script setup lang="ts">
// TOPIC_COMMENT_CREATION — a comment on a reply. Same shape as the reply card:
// the comment body (a few lines), the reply it's on (被评论的评论) quoted above,
// and the topic name anchored at the bottom.
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as TopicCommentActivityData | undefined
)
const quoted = computed(() => data.value?.quoted_reply)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-2">
      <!-- The reply being commented on (被评论的评论). -->
      <ActivityCardQuote
        v-if="quoted"
        :content="quoted.content"
        :label="`#${quoted.floor}`"
      />

      <!-- Body sits OUTSIDE the link so its 显示更多 toggle never navigates;
           preserveNewlines + pre-line keep the author's line breaks. -->
      <ActivityCollapse :max-height="300">
        <p class="text-default-700 text-base break-all whitespace-pre-line">
          {{ markdownToText(activity.content, { preserveNewlines: true }) }}
        </p>
      </ActivityCollapse>

      <KunLink
        v-if="data?.topic_title"
        underline="none"
        color="default"
        :to="commentPermalink(activity.link, data?.comment_id)"
        class-name="text-default-500 hover:text-primary flex items-center gap-1 text-sm"
      >
        <KunIcon name="icon-park-outline:topic" class="size-4 shrink-0" />
        <span class="line-clamp-1">{{ data.topic_title }}</span>
      </KunLink>
    </div>
  </ActivityCardShell>
</template>
