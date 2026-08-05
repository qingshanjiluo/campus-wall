<script setup lang="ts">
// GALGAME_PR_CREATION — a user proposed an update (PR) to a galgame. Mirrors the
// edit card's layout (header + the shared galgame info area); the footer flags it
// as pending review (warning) and links to the PR detail, which shows the full
// proposed diff (kept off the feed card — it'd need a second fetch + assembly).
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as GalgameActivityData | undefined
)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-3">
      <p class="text-default-600 text-sm break-all">
        提出了《{{ data?.name || activity.content }}》的更新请求
      </p>

      <ActivityCardGalgameInfo :activity="activity" />

      <div class="flex items-center justify-between gap-2 text-sm">
        <span class="text-warning-600">该更新请求需要被审核</span>
        <KunLink
          underline="none"
          color="default"
          :to="activity.link"
          class-name="text-default-500 hover:text-primary flex shrink-0 items-center gap-0.5 text-sm"
        >
          查看详情
          <KunIcon name="lucide:chevron-right" class="size-4" />
        </KunLink>
      </div>
    </div>
  </ActivityCardShell>
</template>
