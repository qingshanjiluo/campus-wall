export const asideItems = ref<ChatMessageAsideItem[]>([])

const getAsideMessagePreview = (message: ChatMessage) => {
  if (message.is_recall) {
    return `${message.sender.name}撤回了一条消息`
  }
  return message.content
}

export const replaceAsideItem = (message: ChatMessage) => {
  const targetIndex = asideItems.value.findIndex(
    (item) => item.chatroom_name === message.chatroom_name
  )

  if (targetIndex !== -1) {
    asideItems.value[targetIndex]!.content = getAsideMessagePreview(message)
    asideItems.value[targetIndex]!.last_message_time = message.created
    if (!message.is_recall) {
      asideItems.value[targetIndex]!.count++
    }
  }

  asideItems.value.sort((a, b) => {
    const timeA = new Date(a.last_message_time ?? 0).getTime()
    const timeB = new Date(b.last_message_time ?? 0).getTime()
    return timeB - timeA
  })
}
