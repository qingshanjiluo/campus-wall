<script setup lang="ts">
import { useTopicReplies } from '~/composables/topic/useTopicReplies'
import { useTopicScroll } from '~/composables/topic/useTopicScroll'
import { TOPIC_TOC_SOURCE } from '~/composables/topic/useTopicTOC'

const props = defineProps<{
  topic: TopicDetail
}>()

const { id } = usePersistUserStore()
const canCreateAnyPoll = useCan('poll.create_any')
const canEditAnyPoll = useCan('poll.edit_any')
const tempReplyStore = useTempReplyStore()
const { lastSuccessfulReply } = storeToRefs(tempReplyStore)
// Poll management on this topic: the owner manages their own topic's polls;
// poll.create_any / poll.edit_any extend that to any topic. Drives the poll
// create card + create/edit modal in TopicPollContainer.
const isTopicAdmin = computed(
  () =>
    props.topic.user.id === id || canCreateAnyPoll.value || canEditAnyPoll.value
)

const {
  replies,
  status,
  isComplete,
  hasEarlier,
  sortOrder,
  loadInitialReplies,
  loadMore,
  loadEarlier,
  setSort,
  addNewReply,
  updateReply,
  removeReply
} = useTopicReplies(props.topic.id)

const route = useRoute()
const { scrollToFloor, scrollToComment } = useTopicScroll()

// Deep-link target: /topic/:id?reply=<floor> or ?comment=<id> (proper words; the
// legacy #k<floor> fragment is retired). Locate which reply-stream page it lives
// on so SSR renders that page directly, then scroll + flash on mount.
const targetFloor = Number(route.query.reply) || 0
const targetCommentId = Number(route.query.comment) || 0

let startPage = 1
// The reply floor the rail marks bold: the ?reply floor directly, or the parent
// reply's floor the locate resolves a ?comment to.
let targetReplyFloor = targetFloor
if (targetFloor > 0 || targetCommentId > 0) {
  const located = await kunFetch<{
    page: number
    floor: number
    reply_id: number
    comment_id: number
  }>(`/topic/${props.topic.id}/reply/locate`, {
    query:
      targetCommentId > 0
        ? { comment: targetCommentId }
        : { reply: targetFloor }
  })
  if (located?.page) {
    startPage = located.page
  }
  if (located?.floor) {
    targetReplyFloor = located.floor
  }
}

// The reply the rail bolds + the persistent ring marks. Initialised from the
// locate; a SAME-PAGE ?reply change (clicking a reply's #floor permalink or a
// feed-card link) updates it + re-scrolls without a remount. (?comment only
// arrives via cross-page nav, which remounts → the onMounted branch handles it.)
const activeFloor = ref(targetReplyFloor)
watch(
  () => route.query.reply,
  (value) => {
    const floor = Number(value) || 0
    if (floor > 0) {
      activeFloor.value = floor
      nextTick(() => scrollToFloor(floor, false))
    }
  }
)

await loadInitialReplies(startPage)

onMounted(() => {
  if (!targetFloor && !targetCommentId) {
    return
  }
  // The target's page is already in the SSR HTML; let hydration + initial image
  // layout settle, then scroll + flash. A miss (deleted / off-page) → a hint.
  setTimeout(() => {
    const ok =
      targetCommentId > 0
        ? scrollToComment(targetCommentId, false)
        : scrollToFloor(targetFloor, false)
    if (!ok) {
      useMessage('目标回复或评论可能已被删除', 'info')
    }
  }, 300)
})

provide('topicUserId', props.topic.user.id)
// The active reply floor → Reply.vue draws a PERSISTENT ring on the match (stays
// until you jump elsewhere; no 3s fade like the transient flash).
provide('activeReplyFloor', activeFloor)

// Feed the TOC rail from data so it renders server-side (no flash on refresh);
// the scrollspy inside useTopicTOC stays client-only.
provide(TOPIC_TOC_SOURCE, {
  getContentHtml: () => props.topic.content_html,
  getReplies: () => replies.value,
  getTargetFloor: () => activeFloor.value
})

watch(
  lastSuccessfulReply,
  (event) => {
    if (!event) {
      return
    }

    switch (event.type) {
      case 'created':
        addNewReply(event.data)

        nextTick(() => {
          scrollToFloor(event.data.floor)
        })
        break
      case 'updated':
        updateReply(event.data)

        nextTick(() => {
          scrollToFloor(event.data.floor)
        })
        break
      case 'deleted':
        removeReply(event.data.id)
        break
    }

    tempReplyStore.clearSuccessfulReply()
  },
  { deep: true }
)
</script>

<template>
  <div class="flex flex-col gap-4 lg:flex-row lg:items-start">
    <!-- LEFT: author rail — sticky across the whole page (no card chrome). -->
    <TopicDetailMasterUser v-if="topic.user" :user="topic.user" />

    <!-- RIGHT: post body + 话题小程序 (poll) + tool + replies + action bar. -->
    <div class="min-w-0 flex-1 space-y-4">
      <TopicDetailMaster :topic="topic" />

      <TopicPollContainer :topic-id="topic.id" :is-topic-admin="isTopicAdmin" />

      <!-- scroll-mt offsets the fixed top bar so the 评论 jump button (bottom bar)
           lands the reply-count header at the top, not under the topbar. -->
      <div id="comments-anchor" class="scroll-mt-20">
        <TopicDetailTool
          :reply-count="topic.reply_count"
          :status="status"
          :sort-order="sortOrder"
          @set-sort-order="setSort"
        />
      </div>

      <section id="reply-section" class="space-y-4">
        <!-- A deep-link (?reply / ?comment) can land on a later page; let the
             reader pull in earlier replies (the stream extends upward too). -->
        <div v-if="hasEarlier && status !== 'pending'" class="text-center">
          <KunButton size="lg" variant="flat" @click="loadEarlier">
            加载更早的回复
          </KunButton>
        </div>

        <div
          v-if="status === 'pending' && replies.length === 0"
          class="flex justify-center py-16"
        >
          <KunLoading description="少女祈祷中..." />
        </div>

        <TopicReplyList
          v-else-if="replies.length > 0"
          :initial-replies="replies"
          :topic-id="topic.id"
          :title="topic.title"
        />

        <div class="py-6 text-center">
          <KunButton
            v-if="!isComplete && status !== 'pending'"
            size="lg"
            variant="flat"
            @click="loadMore"
          >
            加载更多
          </KunButton>
          <KunLoading v-if="status === 'pending' && replies.length > 0" />
          <p v-if="isComplete" class="text-default-500">
            {{ `(｡>︿<｡) 已经一滴回复都不剩了哦~` }}
          </p>
        </div>
      </section>

      <TopicDetailActionBar :topic="topic" />
    </div>
  </div>
</template>
