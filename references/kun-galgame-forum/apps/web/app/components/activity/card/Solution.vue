<script setup lang="ts">
// MESSAGE_SOLUTION feed card: a topic owner accepted a reply as the best answer.
// actor = the accepter, content = the accepted reply's preview, data.topic_title =
// the owning topic, link → that topic. Styled in success green to echo the
// in-topic 最佳答案 banner.
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as SolutionActivityData | undefined
)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div
      class="text-success-600 dark:text-success-400 flex items-center gap-1.5 text-sm font-medium"
    >
      <KunIcon name="lucide:bookmark-check" class-name="text-base" />
      采纳了最佳答案
    </div>

    <!-- The accepted answer's preview — green fill, all corners rounded. -->
    <div class="bg-success-500/10 mt-2 rounded-lg p-3">
      <KunText
        class-name="whitespace-normal! text-default-600 line-clamp-3 text-sm"
        :content="markdownToText(activity.content)"
      />
    </div>

    <!-- The owning topic's name + a 查看详情 link to it. -->
    <div
      class="text-default-500 mt-2 flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm"
    >
      <KunLink
        v-if="data?.topic_title"
        underline="hover"
        color="default"
        :to="replyPermalink(activity.link, data?.floor)"
        class-name="text-default-500 hover:text-primary inline-flex min-w-0 items-center gap-1.5"
      >
        <KunIcon name="icon-park-outline:topic" class-name="shrink-0" />
        <span class="truncate">{{ data.topic_title }}</span>
      </KunLink>
      <span v-else />

      <KunLink
        underline="none"
        color="default"
        :to="replyPermalink(activity.link, data?.floor)"
        class-name="text-default-500 hover:text-primary flex shrink-0 items-center gap-0.5 text-sm"
      >
        查看详情
        <KunIcon name="lucide:chevron-right" class="size-4" />
      </KunLink>
    </div>
  </ActivityCardShell>
</template>
