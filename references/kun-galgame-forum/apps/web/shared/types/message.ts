export type MessageType =
  | 'upvoted'
  | 'liked'
  | 'favorite'
  | 'replied'
  | 'solution'
  | 'pin-reply'
  | 'commented'
  | 'expired'
  | 'requested'
  | 'merged'
  | 'declined'
  | 'mentioned'
  | 'admin'
  | 'quiz-answered'

type MessageStatus = 'read' | 'unread'

// Per-user notification preferences (BE migration 053). The opt-out set of
// muted category keys: message.type values, the "system"/"chat" stream pseudo
// keys, and namespaced "wiki:*" keys. Muting only suppresses the red dot /
// unread badges — the messages themselves are still kept.
export interface NotificationPreference {
  muted_types: string[]
}

type MessageSortField = 'time'

// Request query params stay camelCase (backend binds query:"sortField"/"sortOrder").
export interface MessageRequestData {
  page: string
  limit: string
  type?: MessageType | ''
  sortField?: MessageSortField
  sortOrder: KunOrder
}

export interface Message {
  id: number
  sender: KunUser
  receiver_id: number
  link: string
  content: string
  status: MessageStatus
  type: MessageType
  created: Date | string
}

// Wire shape of GET /message (BE dto.MessageListResponse). FE
// pages/message/notice.vue previously referenced an undeclared
// `MessageList` and fell through to TS `any`.
export interface MessageList {
  messages: Message[]
  total: number
}

// System broadcasts use a per-user HWM cursor (system_message_read_state)
// instead of the legacy row-level `status` field. The BE evaluates
// `isRead = id <= cursor` for the caller — see migration 012.
export interface MessageSystemMessage {
  id: number
  is_read: boolean
  content: KunNullable<KunLanguage>
  admin: KunUser
  created: Date | string
}
