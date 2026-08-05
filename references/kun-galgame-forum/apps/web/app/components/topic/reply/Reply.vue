<script setup lang="ts">
import { scrollPage } from '../_helper'
import { useQuoteContent } from '~/composables/topic/useQuoteContent'

const bannerBaseClasses =
  'flex items-center gap-2 px-4 py-2 mb-3 rounded-lg font-semibold text-sm'

const props = defineProps<{
  reply: TopicReply
  title: string
}>()

const { scrollToReplyId } = storeToRefs(useTempReplyStore())
const comments = ref(props.reply.comment)

// The deep-link / #floor target (provided by Detail) gets a PERSISTENT ring. The
// wrapper already carries outline-primary + offset, so being active just toggles
// the outline width + rounding — it stays until the reader jumps to another reply.
const activeFloor = inject('activeReplyFloor', ref(0))
const isActive = computed(
  () => activeFloor.value > 0 && activeFloor.value === props.reply.floor
)

// Shared reaction state for this reply — injected by the chips + the trigger.
provide(
  reactionsKey,
  useReactions({
    replyId: props.reply.id,
    targetUserId: props.reply.user.id,
    reactions: props.reply.reactions,
    showReactors: true
  })
)

// Hydrate inline #quote chips in this reply's body: click → jump to the floor,
// hover → lazy preview card.
const contentRef = ref<HTMLElement | null>(null)
const { preview, keepPreview, hidePreview } = useQuoteContent(contentRef)

// Short plain-text slug for the reply's anchor id. markdownToText first so a
// mention/quote token isn't sliced mid-form into the id.
const replyContent = computed(() =>
  markdownToText(props.reply.content_markdown).slice(0, 20)
)

const cardClasses = computed(() => {
  if (props.reply.is_best_answer) {
    return 'border-l-4 border-success-600 dark:border-success-700'
  }
  if (props.reply.is_pinned) {
    return 'border-l-4 border-secondary-500 dark:border-secondary-600'
  }
  return ''
})

watch(
  () => scrollToReplyId.value,
  async () => {
    if (scrollToReplyId.value !== -1) {
      await nextTick()
      scrollPage(scrollToReplyId.value)
      scrollToReplyId.value = -1
    }
  }
)

const handleNewComment = (comment: TopicComment) => {
  comments.value.push(comment)
}
</script>

<template>
  <div
    :class="
      cn(
        'outline-primary kun-reply flex justify-between gap-3 outline-offset-2',
        isActive && 'outline-2 rounded-lg'
      )
    "
    :id="`${reply.floor}.${replyContent}`"
  >
    <KunCard
      :is-transparent="false"
      :is-hoverable="false"
      :class-name="cn('w-full min-w-0 relative overflow-visible', cardClasses)"
      content-class="gap-3"
    >
      <div
        v-if="reply.is_best_answer"
        :class="
          cn(
            'bg-success-500/20 text-success-700 dark:text-success-300',
            bannerBaseClasses
          )
        "
      >
        <KunIcon class-name="text-xl" name="lucide:bookmark-check" />
        <span>最佳答案</span>
        <KunIcon
          class-name="absolute bottom-3 right-3 text-[10rem] text-success-500/20 select-none -z-1"
          name="lucide:circle-check-big"
        />
      </div>

      <div
        v-else-if="reply.is_pinned"
        :class="
          cn(
            'bg-secondary-500/20 text-secondary-700 dark:text-secondary-300',
            bannerBaseClasses
          )
        "
      >
        <KunIcon class-name="text-xl" name="lucide:pin" />
        <span>置顶回复</span>
        <KunIcon
          class-name="absolute bottom-3 right-3 text-[10rem] text-secondary-500/20 select-none -z-1"
          name="lucide:disc-2"
        />
      </div>

      <TopicDetailUser
        :user="reply.user"
        :created="reply.created"
        :edited="reply.edited"
        :topic-id="reply.topic_id"
        :floor="reply.floor"
      />

      <div ref="contentRef">
        <KunContent
          v-if="reply.content_markdown && reply.content_markdown.trim()"
          compact
          :content="renderKatex(reply.content_html)"
        />
      </div>

      <TopicQuotePreview
        :preview="preview"
        @keep="keepPreview"
        @leave="hidePreview"
      />

      <div class="mt-2 flex flex-wrap items-center gap-1.5">
        <TopicReactionBar />
        <TopicReactionTrigger />
      </div>

      <TopicReplyFooter
        :reply="reply"
        :title="title"
        @handle-new-comment="handleNewComment"
      />

      <TopicComment :reply-id="reply.id" :comments-data="comments" />
    </KunCard>
  </div>
</template>
