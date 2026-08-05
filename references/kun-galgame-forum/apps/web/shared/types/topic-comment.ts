export interface TopicComment {
  id: number
  reply_id: number
  topic_id: number
  // The comment this one replies to (nested comments); null/undefined = a
  // top-level comment attached to the reply directly.
  parent_comment_id?: number | null
  user: KunUser
  target_user: KunUser
  content: string
  is_liked: boolean
  like_count: number
  created: Date | string
  // set only when the author edits the comment; drives the "(编辑于 …)" hint
  edited?: Date | string | null
}
