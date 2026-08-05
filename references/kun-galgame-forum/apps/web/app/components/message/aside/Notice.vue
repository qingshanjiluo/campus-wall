<script setup lang="ts">
import { getMessageI18n } from '../utils/getMessageI18n'

const props = defineProps<{
  message: Message
  refresh: () => void
}>()

// The content preview is the linked entity's title (galgame / topic name).
// Some rows legitimately carry no content — older `merged` notices written
// before the name was captured, or a galgame that has no localized name yet —
// which rendered as a blank, oddly-tall line that "shows no game". Fall back to
// a clickable label so the row always points somewhere meaningful.
const contentPreview = computed(() => {
  const text = markdownToText(props.message.content).trim()
  return text || '点击查看详情'
})

const handleDeleteMessage = async (messageId: number) => {
  const res = await useComponentMessageStore().alert(
    '您确定要删除这条消息吗？此操作不可撤销。'
  )
  if (!res) {
    return
  }

  const result = await kunFetch(`/message/${messageId}`, {
    method: 'DELETE'
  })

  if (result) {
    props.refresh()
    useMessage(10106, 'success')
  }
}
</script>

<template>
  <div
    class="space-y-2 rounded-lg p-2"
    :class="message.status === 'read' ? 'message-read' : ''"
  >
    <div class="flex items-center gap-2 break-all">
      <div class="flex text-lg">
        <KunIcon
          class="text-secondary"
          v-if="message.status === 'unread'"
          name="lucide:info"
        />
        <KunIcon
          class="text-default"
          v-if="message.status === 'read'"
          name="lucide:check-check"
        />
      </div>

      <div>
        <KunLink :to="`/user/${message.sender.id}`">
          {{ message.sender.name }}
        </KunLink>
        <span>{{ getMessageI18n(message) }}</span>
      </div>
    </div>

    <div class="flex gap-2">
      <KunAvatar :disable-floating="true" :user="message.sender" />

      <KunLink
        color="default"
        underline="none"
        class="hover:text-primary cursor-pointer transition-colors"
        :to="message.link"
      >
        <pre
          class="break-word text-sm leading-8 whitespace-pre-line text-inherit"
          >{{ contentPreview }}</pre
        >
      </KunLink>
    </div>

    <div class="flex justify-between">
      <span class="text-default-500 text-sm">
        <KunTime :time="message.created" type="datetime" show-year />
      </span>

      <KunButton
        :is-icon-only="true"
        variant="light"
        color="danger"
        size="sm"
        @click="handleDeleteMessage(message.id)"
      >
        <KunIcon name="lucide:trash-2" />
      </KunButton>
    </div>
  </div>
</template>
