<script setup lang="ts">
defineProps<{
  title: string
  reply: TopicReply
}>()

const emits = defineEmits<{
  handleNewComment: [comment: TopicComment]
}>()

const { id } = usePersistUserStore()
const isCommentPanelVisible = ref(false)

const handleClickComment = () => {
  if (!id) {
    useAuthModal().open()
    return
  }
  isCommentPanelVisible.value = !isCommentPanelVisible.value
}

const handleNewComment = (comment: TopicComment) => {
  emits('handleNewComment', comment)
  isCommentPanelVisible.value = false
}
</script>

<template>
  <div class="w-full">
    <div class="flex items-center justify-end">
      <div class="flex items-center gap-1">
        <TopicFooterReply
          :target-user-name="reply.user.name"
          :target-user-id="reply.user.id"
          :target-floor="reply.floor"
          :target-reply-id="reply.id"
        />
        <TopicReplyRewrite :reply="reply" />
        <KunTooltip text="评论">
          <KunReaction
            :toggle="false"
            icon="uil:comment-dots"
            label="评论"
            @click="handleClickComment"
          />
        </KunTooltip>
        <!-- ... 更多按钮 (分享 + 置顶/最佳/删除) ... -->
        <KunPopover position="top-end">
          <template #trigger>
            <KunReaction :toggle="false" icon="lucide:ellipsis" label="更多" />
          </template>

          <div class="flex w-54 flex-col gap-2 p-2">
            <KunButton
              variant="light"
              color="default"
              size="sm"
              class-name="w-full justify-start gap-2 whitespace-nowrap"
              @click="
                useKunCopy(
                  `${title}: https://www.kungal.com/topic/${reply.topic_id}?reply=${reply.floor}`
                )
              "
            >
              <KunIcon class-name="text-lg" name="lucide:share-2" />
              分享该回复
            </KunButton>
            <template v-if="id">
              <TopicReplyPin :reply="reply" />
              <TopicReplyBestAnswer :reply="reply" />
              <TopicReplyDelete :reply="reply" />
            </template>
            <ReportButton
              v-if="reply.user.id !== id"
              menu
              subject-kind="forum_reply"
              :subject-id="reply.id"
              :snapshot="reply.content_markdown"
              :subject-url="`${kungal.domain.main}/topic/${reply.topic_id}?reply=${reply.floor}`"
            />
          </div>
        </KunPopover>
      </div>
    </div>

    <KunFadeCard>
      <LazyTopicCommentPanel
        v-if="isCommentPanelVisible"
        class="mt-4"
        :reply-id="reply.id"
        :target-user="reply.user"
        @get-comment="handleNewComment"
        @close-panel="isCommentPanelVisible = false"
      />
    </KunFadeCard>
  </div>
</template>
