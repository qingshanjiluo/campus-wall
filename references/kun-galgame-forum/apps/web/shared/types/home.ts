import type { GalgameCard } from './galgame.d.ts'

export interface HomeUserStatus {
  moemoepoints: number
  isCheckIn: boolean
  hasNewMessage: boolean
}

export interface HomeTopic {
  id: number
  title: string
  view: number

  section: string[]
  // Optional 1..9 feed-card cover images, as /image/<hash> content tokens
  // (usable directly as an <img src>). Empty array = no covers.
  cover_images: string[]
  // Per-cover-token metadata (dims + ThumbHash), keyed by the /image/<hash>
  // token in cover_images — for no-CLS aspect ratio + blur-up. Absent pre-backfill.
  cover_image_meta?: Record<string, KunImageMeta>
  user: KunUser
  status: number
  has_best_answer: boolean
  is_poll_topic: boolean
  is_nsfw_topic: boolean

  like_count: number
  reply_count: number
  comment_count: number

  status_update_time: Date | string
  upvote_time: Date | string | null
}

export type HomeGalgame = GalgameCard
