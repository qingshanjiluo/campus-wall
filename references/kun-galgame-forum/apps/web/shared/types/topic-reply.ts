import type { TopicComment } from './topic-comment'

export interface TopicReply {
  id: number
  topic_id: number
  floor: number
  user: KunUser & { moemoepoint: number }
  content_markdown: string
  content_html: string

  like_count: number
  is_liked: boolean
  dislike_count: number
  is_disliked: boolean
  reactions: KunReaction[]

  comment: TopicComment[]
  created: Date | string
  edited: Date | string | null

  is_pinned: boolean
  is_best_answer: boolean
}

