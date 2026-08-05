<script setup lang="ts">
const { id } = usePersistUserStore()
const tempReplyStore = useTempReplyStore()
const { isEdit } = storeToRefs(tempReplyStore)

const props = defineProps<{
  targetUserName: string
  targetUserId: number
  targetFloor: number
  targetReplyId?: number
}>()

const handleClickReply = () => {
  if (!id) {
    useAuthModal().open()
    return
  }

  // Replying to a specific floor: hand the editor a 「引用」 signal, which it turns
  // into an inline `@author #floor` header (mention notifies the author, quote
  // links the floor) via a live editor command — caret lands on the body line
  // below, WYSIWYG. Replying to the topic (floor 0) just opens the panel.
  if (props.targetFloor !== 0 && props.targetReplyId) {
    tempReplyStore.setPendingQuote({
      userId: props.targetUserId,
      userName: props.targetUserName,
      replyId: props.targetReplyId,
      floor: props.targetFloor
    })
  }

  isEdit.value = true
}
</script>

<template>
  <KunReaction :toggle="false" icon="lucide:reply" @click="handleClickReply">
    回复
  </KunReaction>
</template>
