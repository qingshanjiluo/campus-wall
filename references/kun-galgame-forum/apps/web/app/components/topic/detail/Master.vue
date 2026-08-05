<script setup lang="ts">
const props = defineProps<{
  topic: TopicDetail
}>()

// Covers the reader would otherwise never see.
//
// A cover is a FEED-CARD affordance — the thumbnail that earns the click in a
// list — and by the time someone is on this page it has already done its job.
// It is also, 97% of the time, literally the first image of the body (authors
// re-upload the opening image as the cover), so rendering covers unconditionally
// would show the same picture twice within one screen.
//
// So only the covers absent from the body are shown: the ~2.6% where the author
// deliberately picked a different image, and the handful whose body carries no
// image at all. For everyone else this renders nothing, which is correct —
// nothing was missing there.
const unseenCovers = computed(() => {
  const body = props.topic.content_markdown ?? ''
  return (props.topic.cover_images ?? []).filter((token) => {
    const hash = token.split('/').pop()
    return hash ? !body.includes(hash) : false
  })
})

// Shared reaction state for this topic: the chips (below) + the trigger (in the
// footer on desktop) both inject this, so reacting in either place stays in sync.
provide(
  reactionsKey,
  useReactions({
    topicId: props.topic.id,
    targetUserId: props.topic.user.id,
    reactions: props.topic.reactions,
    showReactors: true
  })
)
</script>

<template>
  <div id="0" class="outline-primary rounded-lg outline-offset-2">
    <KunCard
      :is-transparent="false"
      :is-hoverable="false"
      class-name="w-full min-w-0"
      content-class="gap-4 justify-start"
    >
      <!-- Post header — the title leads the hierarchy (larger than any in-body
           heading), with categorization chips and a compact icon byline below. -->
      <header class="space-y-3">
        <h1
          class="text-3xl leading-tight font-bold tracking-tight break-words lg:text-4xl"
        >
          {{ topic.title }}
        </h1>

        <TopicTagGroup
          :section="topic.section"
          :upvote-time="topic.upvote_time"
          :has-best-answer="false"
          :is-poll-topic="topic.is_poll_topic"
          :is-n-s-f-w-topic="topic.is_nsfw"
          :is-nav-to-section="true"
        />

        <div
          class="text-default-500 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm"
        >
          <span class="flex items-center gap-1.5">
            <KunIcon name="lucide:eye" class="size-4" />
            {{ topic.view }}
          </span>
          <span class="flex items-center gap-1.5">
            <KunIcon name="lucide:clock" class="size-4" />
            <KunTime :time="topic.created" type="datetime" show-year />
          </span>
          <span v-if="topic.edited" class="flex items-center gap-1.5">
            <KunIcon name="lucide:pencil-line" class="size-4" />
            编辑于 <KunTime :time="topic.edited" type="datetime" show-year />
          </span>
        </div>
      </header>

      <TopicDetailBestAnswer
        v-if="topic.best_answer"
        :best-answer="topic.best_answer"
      />

      <TopicDetailUser
        class-name="lg:hidden"
        :user="topic.user"
        :created="topic.created"
        :edited="topic.edited"
        :topic-id="topic.id"
        :floor="0"
        :show-addition="false"
      />

      <KunDivider />

      <!-- Lead image, only when the body does not already contain it (see
           unseenCovers). Reuses the feed card's grid, which already handles
           multi-cover layout and reserves each slot from the real dims. -->
      <TopicCoverGrid
        v-if="unseenCovers.length"
        :images="unseenCovers"
        :meta="topic.cover_image_meta"
        zoomable
      />

      <KunContent
        class="kun-master"
        :content="renderKatex(topic.content_html)"
      />

      <KunDivider />

      <!-- 推话题 records: who pushed this topic + their one-liner + how long ago.
           Sits between the divider and the reaction row. -->
      <TopicUpvoteRecords :topic-id="topic.id" />

      <div class="flex flex-wrap items-center gap-1.5">
        <TopicReactionBar />
        <!-- Desktop shows the trigger in the footer (next to favorite). The wrapper
             carries md:hidden because the trigger renders a fragment (popover +
             history modal) and can't inherit the class itself. -->
        <span class="md:hidden">
          <TopicReactionTrigger />
        </span>
      </div>

      <p class="text-default-500 ml-auto text-sm">
        本文版权遵循
        <KunLink
          underline="hover"
          size="sm"
          class-name="text-default-500"
          target="_blank"
          rel="noopener noreferrer"
          to="https://creativecommons.org/licenses/by-nc/4.0/deed.en"
        >
          CC BY-NC 协议
        </KunLink>
        和
        <KunLink
          underline="hover"
          size="sm"
          class-name="text-default-500"
          to="/doc/article-copyright"
        >
          本站版权政策
        </KunLink>
      </p>

      <TopicFooter :topic="topic" />
    </KunCard>
  </div>
</template>
