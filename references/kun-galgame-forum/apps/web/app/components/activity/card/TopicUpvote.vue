<script setup lang="ts">
// 推话题 (TOPIC_UPVOTE) — a user pushed a topic. The shell shows who (avatar +
// name + time); this card adds:
//   top    — 推了这个话题，<blurb> (the user's one-liner, or a stable random
//            default, in secondary — the playful "why I pushed it")
//   middle — the pushed topic's title + preview (links to the topic)
//   bottom — the same footer as the new-topic card (收藏 + reactions · 浏览 +
//            查看详情). Reuses the topic enrichment payload (data), filled by the
//            backend from the upvote's topic id.
import { randomUpvoteDescription } from '~/constants/upvote'

const props = defineProps<{ activity: ActivityItem }>()

const data = computed(
  () => props.activity.data as TopicActivityData | undefined
)
const topicId = computed(() => data.value?.topic_id ?? 0)
const covers = computed(() => (data.value?.cover_images ?? []).slice(0, 3))

// The blurb: the user's one-liner, else a stable random default seeded by the
// upvote id (uniqueId = "TOPIC_UPVOTE:<id>") — varies across the feed, never
// flickers per item.
const seed = computed(() => {
  const n = Number(props.activity.unique_id.split(':').pop())
  return Number.isFinite(n) ? n : topicId.value
})
const blurb = computed(
  () => props.activity.content || randomUpvoteDescription(seed.value)
)

// Per-viewer 收藏 + reaction state (the shared feed can't carry it).
const { isFavorited, reactionKeysOf, ensureLoaded } = useMyTopicInteractions()
onMounted(ensureLoaded)

const reactionList = computed<KunReaction[]>(() =>
  (data.value?.reactions ?? []).map((r) => ({
    reaction: r.reaction,
    count: r.count,
    reactors: r.reactors,
    mine: reactionKeysOf(topicId.value).includes(r.reaction)
  }))
)
provide(
  reactionsKey,
  useReactions({
    topicId: topicId.value,
    targetUserId: data.value?.author_id ?? 0,
    reactions: reactionList.value,
    sync: () => reactionList.value,
    showReactors: true
  })
)
</script>

<template>
  <ActivityCardShell :actor="activity.actor" :timestamp="activity.timestamp">
    <div class="space-y-3">
      <p class="text-default-600 text-sm break-all">
        推了这个话题，<span class="text-secondary font-bold">{{ blurb }}</span>
      </p>

      <KunLink
        underline="none"
        color="default"
        :to="activity.link"
        class-name="group block space-y-2.5"
      >
        <h3
          class="group-hover:text-primary line-clamp-2 text-lg font-medium break-all transition-colors"
        >
          {{ data?.title }}
        </h3>
        <p
          v-if="data?.excerpt"
          class="text-default-500 line-clamp-3 text-sm break-all"
        >
          {{ markdownToText(data.excerpt) }}
        </p>
        <TopicCoverGrid
          v-if="covers.length"
          :images="covers"
          :meta="data?.cover_image_meta"
        />
      </KunLink>

      <!-- Footer: 收藏 + reactions (clickable) · 浏览 + 查看详情. -->
      <div class="flex items-center justify-between gap-2">
        <div class="flex min-w-0 flex-wrap items-center gap-1">
          <TopicFooterFavorite
            :topic-id="topicId"
            :favorite-count="data?.favorite_count ?? 0"
            :is-favorite="isFavorited(topicId)"
          />
          <TopicReactionBar />
          <TopicReactionTrigger />
        </div>

        <div class="text-default-500 flex shrink-0 items-center gap-3 text-sm">
          <span class="flex items-center gap-1">
            <KunIcon name="lucide:eye" class="size-4" />
            {{ formatNumber(data?.view ?? 0) }}
          </span>
          <KunLink
            underline="none"
            color="default"
            :to="activity.link"
            class-name="text-default-500 hover:text-primary flex items-center gap-0.5 text-sm"
          >
            查看详情
            <KunIcon name="lucide:chevron-right" class="size-4" />
          </KunLink>
        </div>
      </div>
    </div>
  </ActivityCardShell>
</template>
