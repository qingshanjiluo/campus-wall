<script setup lang="ts">
const props = defineProps<{
  reply: TopicReply
}>()

const { setRewriteData } = useTempReplyStore()
const { isEdit } = storeToRefs(useTempReplyStore())
const { id } = usePersistUserStore()

const isShowRewrite = computed(() => id === props.reply.user.id)

const handleClickRewrite = () => {
  setRewriteData(props.reply)
  isEdit.value = true
}
</script>

<template>
  <KunTooltip v-if="isShowRewrite" text="重新编辑">
    <KunReaction
      :toggle="false"
      icon="lucide:pencil"
      label="重新编辑"
      @click="handleClickRewrite"
    />
  </KunTooltip>
</template>
