export type ActivityEventType =
  | 'GALGAME_CREATION'
  | 'GALGAME_COMMENT_CREATION'
  | 'GALGAME_RATING_CREATION'
  | 'GALGAME_RATING_COMMENT_CREATION'
  | 'GALGAME_PR_CREATION'
  | 'GALGAME_EDIT'
  | 'GALGAME_WEBSITE_CREATION'
  | 'GALGAME_WEBSITE_COMMENT_CREATION'
  | 'GALGAME_RESOURCE_CREATION'
  | 'GALGAME_QUIZ_CREATION'
  | 'TOOLSET_CREATION'
  | 'TOOLSET_RESOURCE_CREATION'
  | 'TOOLSET_COMMENT_CREATION'
  | 'TOPIC_CREATION'
  | 'TOPIC_REPLY_CREATION'
  | 'TOPIC_COMMENT_CREATION'
  | 'TOPIC_UPVOTE'
  | 'TODO_CREATION'
  | 'UPDATE_LOG_CREATION'
  | 'MESSAGE_UPVOTE'
  | 'MESSAGE_SOLUTION'

// A topic's most-liked reply (excerpt + like count), shown on the topic card.
export interface ActivityTopReply {
  // The reply's id, so the card can tell if 高赞回复 is also the best answer.
  reply_id: number
  // The reply's floor → deep-link to it (/topic/:id?reply=<floor>).
  floor: number
  user: KunUser
  content: string
  like_count: number
}

// Rich-card payload for TOPIC_CREATION (BE dto.TopicActivityData). The title
// lives in `content`; this carries the extras the topic feed card shows.
export interface TopicActivityData {
  topic_id: number
  // The 推话题 (TOPIC_UPVOTE) card reads the title here (its `content` carries the
  // push description); TOPIC_CREATION uses `content` for the title.
  title?: string
  // The topic author's id (reaction target on the 推话题 card, whose actor is the
  // pusher, not the author).
  author_id?: number
  excerpt: string
  sections: string[]
  cover_images: string[]
  // Per-cover-token metadata (dims + ThumbHash), keyed by the /image/<hash>
  // token in cover_images — for no-CLS aspect ratio + blur-up. Absent pre-backfill.
  cover_image_meta?: Record<string, KunImageMeta>
  view: number
  like_count: number
  favorite_count: number
  reply_count: number
  comment_count: number
  upvote_time: Date | string | null
  // The topic's last edit time (null = never edited); the card shows an edit icon
  // + relative time after the timestamp.
  edited?: Date | string | null
  has_best_answer: boolean
  is_poll: boolean
  is_nsfw: boolean
  top_reply?: ActivityTopReply
  // The accepted best answer (omitted when none). Same reply as top_reply (same
  // reply_id) → the card shows only the best-answer style.
  best_answer?: ActivityTopReply
  // 推话题 records (all of them — few per topic); same shape the topic-detail
  // 推话题 list consumes, so the card reuses TopicUpvoteRecords.
  upvotes?: {
    id: number
    user: KunUser
    description: string
    created: Date | string
  }[]
  // The topic's newest reply or comment (omitted when none). kind 'reply' carries
  // its reply_id so the card can merge it when it's the best answer / 高赞回复.
  latest_activity?: {
    kind: 'reply' | 'comment'
    reply_id: number
    // floor (reply) / comment_id (comment) → deep-link to the target.
    floor: number
    comment_id: number
    user: KunUser
    content: string
    created: Date | string
  }
  // Reaction counts per key + up to 3 reactor avatars (shared/cacheable). The
  // viewer's own "mine" is hydrated separately via useMyTopicInteractions.
  reactions: { reaction: string; count: number; reactors?: KunUser[] }[]
}

// Rich-card payload for galgame-scoped activity (BE dto.GalgameActivityData).
// cover_hash resolves to a CDN URL via imageTokenUrl. The count fields are only
// populated for the GALGAME_CREATION card (global counts; the viewer's own
// liked/favorited state is NOT carried — it would break the shared feed cache).
export interface GalgameActivityData {
  name: string
  cover_hash: string
  language: string
  age_limit: string
  release_date: string | null
  galgame_id?: number
  resource_count?: number
  like_count?: number
  favorite_count?: number
  // GALGAME_EDIT only. revision_number = the per-galgame revision number the diff
  // endpoint's :rev keys on (used directly); revision_id = the wiki revision ROW
  // id, the legacy fallback resolved id→number for rows synced before the feed
  // carried the number.
  revision_id?: number
  revision_number?: number
  // GALGAME_CREATION + GALGAME_EDIT (shared info area), from the wiki detail
  // brief: developer = 制作会社 (officials joined with 、); intro =
  // preferred-language introduction.
  developer?: string
  intro?: string
  // GALGAME_RATING_CREATION only — the rating card's fields.
  rating?: ActivityRatingInfo
  // GALGAME_COMMENT_CREATION — the parent comment being replied to (被评论的评论).
  parent_comment?: { content: string }
  // GALGAME_RESOURCE_CREATION — the published resource's spec (download link /
  // 提取码 / 解压码 deliberately omitted).
  resource?: {
    type: string
    language: string
    platform: string
    size: string
    note: string
    like_count: number
  }
}

// Rating card payload (BE dto.RatingInfo). short_summary is blank when spoiler_level
// !== 'none' (the card shows a spoiler notice instead).
export interface ActivityRatingInfo {
  rating_id: number
  overall: number
  play_status: string
  recommend: string
  short_summary: string
  spoiler_level: string
  like_count: number
  author_id: number
}

// The reply this reply quoted (#floor → its body), shown as a nested block.
export interface ActivityQuotedReply {
  floor: number
  content: string
}

// Rich-card payload for TOPIC_REPLY_CREATION (BE dto.ReplyActivityData). The
// reply body is in `content` (tokens already resolved to @name / #floor).
export interface ReplyActivityData {
  topic_title: string
  // This reply's floor → deep-link to it (/topic/:id?reply=<floor>).
  floor: number
  quoted_reply?: ActivityQuotedReply
}

// Rich-card payload for TOPIC_COMMENT_CREATION (BE dto.TopicCommentActivityData).
// The comment is on a reply — quoted_reply is that reply (被评论的评论), topic_title
// anchors the bottom; the comment body is in ActivityItem.content.
export interface TopicCommentActivityData {
  topic_title: string
  // This comment's id → deep-link to it (/topic/:id?comment=<id>).
  comment_id: number
  quoted_reply?: ActivityQuotedReply
}

// Rich-card payload for the 其他-tab Note card. UPDATE_LOG_CREATION carries the
// changelog version; TODO_CREATION carries the completion status (0待处理 …).
export interface NoteActivityData {
  version?: string
  status?: number
}

// Parent-entity name for the toolset/website resource + comment cards (the
// creation cards carry the name in `content` and need no payload).
export interface EntityRefActivityData {
  parent_name: string
}

// Rich-card payload for MESSAGE_SOLUTION (BE dto.SolutionActivityData): the title
// of the topic whose best answer was accepted; the accepted reply's preview is in
// ActivityItem.content.
export interface SolutionActivityData {
  topic_title: string
  // The accepted reply's floor → deep-link to it (/topic/:id?reply=<floor>).
  floor: number
}

// Rich-card payload for GALGAME_QUIZ_CREATION (出题): the quiz's category / type /
// difficulty + answer stats. The 题干 is in ActivityItem.content.
export interface QuizActivityData {
  category: string
  type: string
  difficulty: number
  answer_count: number
  correct_count: number
  favorite_count: number
  // The 题目描述, truncated to a 200-char markdown preview (blank when none).
  description: string
}

// Per-type rich-card payload, discriminated by ActivityItem.type. Each card
// casts activity.data to the shape its type carries (the dispatcher routes by
// type, so the cast is safe). Absent for types without a rich card yet.
export type ActivityData =
  | TopicActivityData
  | GalgameActivityData
  | ReplyActivityData
  | TopicCommentActivityData
  | NoteActivityData
  | EntityRefActivityData
  | SolutionActivityData
  | QuizActivityData

export interface ActivityItem {
  unique_id: string
  type: ActivityEventType
  timestamp: Date | string
  actor: KunUser
  link: string
  content: string
  data?: ActivityData
}
