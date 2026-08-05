export interface ChatMessageHistoryRequest {
  // Request query params stay camelCase (backend binds query:"receiverId");
  // response fields are the ones that were snake_cased.
  receiverId: string
  page: string
  limit: string
}

export interface ChatMessageAsideItem {
  chatroom_name: string
  content: string
  count: number
  unread_count: number
  route: string
  title: string
  avatar: string
  // BE returns null/empty when no message has been exchanged yet
  // (NavContactItem.LastMessageTime *string + the nav/system summary
  // emits ""). The Item/SystemItem templates already guard with v-if,
  // so allowing null/"" here removes the formatTimeDifference("") →
  // "NaN years ago" rendering on fresh chat rooms.
  last_message_time: Date | string | null
}

export interface ChatMessage {
  id: number
  chatroom_name: string
  sender: KunUser
  read_by: KunUser[]
  receiver_id: number
  // Raw markdown source (used for editing/quoting). Render content_html, not
  // this, in the bubble.
  content: string
  // Server-rendered, sanitized inline+image HTML — safe to v-html.
  content_html: string
  is_recall: boolean
  created: Date | string
  recall_time: Date | string | null
  edit_time: Date | string | null
}
