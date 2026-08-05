<script setup lang="ts">
// GALGAME_COMMENT_CREATION — a comment on a galgame. Per spec: comment text on
// top, the galgame name below (linked). No bordered preview box / banner — the
// comment is the focus, the game is just a small reference line.
const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as GalgameActivityData | undefined
)
const gid = computed(() => data.value?.galgame_id ?? 0)
const detailLink = computed(() =>
  gid.value ? `/galgame/${gid.value}` : props.activity.link
)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-1.5">
      <!-- The comment being commented on (被评论的评论). -->
      <ActivityCardQuote
        v-if="data?.parent_comment"
        :content="data.parent_comment.content"
      />

      <!-- Full comment body, rendered as Markdown (server-rendered HTML) — same
           renderer as the galgame detail; untruncated. -->
      <KunContent
        compact
        class="text-base"
        :content="renderKatex(activity.content)"
      />
      <KunLink
        underline="none"
        color="default"
        :to="detailLink"
        class-name="text-default-500 hover:text-primary inline-flex items-center gap-1 text-sm"
      >
        <KunIcon name="lucide:gamepad-2" class="size-3.5 shrink-0" />
        {{ data?.name }}
      </KunLink>
    </div>
  </ActivityCardShell>
</template>
