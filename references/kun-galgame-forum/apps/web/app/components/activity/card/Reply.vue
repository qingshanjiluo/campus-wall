<script setup lang="ts">
// Rich feed card for TOPIC_REPLY_CREATION:
//   top    — username + time (shell)
//   middle — the reply body (a few lines); if it quoted another reply, that
//            quoted reply shows as a nested block above the body
//   bottom — the title of the topic the reply is in
// content + the quoted body arrive with @/# tokens already resolved (BE).
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(() => props.activity.data as ReplyActivityData | undefined)
const quoted = computed(() => data.value?.quoted_reply)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-2">
      <!-- The reply being replied to (quoted #floor). -->
      <ActivityCardQuote
        v-if="quoted"
        :content="quoted.content"
        :label="`#${quoted.floor}`"
      />

      <!-- Full reply body, rendered as Markdown (server-rendered HTML, same
           renderer as the topic detail) — untruncated, NOT inside the link. -->
      <KunContent
        compact
        class="text-base"
        :content="renderKatex(activity.content)"
      />

      <KunLink
        v-if="data?.topic_title"
        underline="none"
        color="default"
        :to="replyPermalink(activity.link, data?.floor)"
        class-name="text-default-500 hover:text-primary flex items-center gap-1 text-sm"
      >
        <KunIcon name="icon-park-outline:topic" class="size-4 shrink-0" />
        <span class="line-clamp-1">{{ data.topic_title }}</span>
      </KunLink>
    </div>
  </ActivityCardShell>
</template>
