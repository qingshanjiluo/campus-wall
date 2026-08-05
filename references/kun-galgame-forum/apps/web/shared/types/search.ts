import type { HomeTopic, HomeGalgame } from './home'

export type SearchResultTopic = HomeTopic
export type SearchResultGalgame = HomeGalgame

// BE `UserItem` (apps/api/internal/search/dto) currently leaves
// `moemoepoint` and `created` zero — they're not joined from
// kungal_user_state at search time. Type them as optional so the FE
// template can guard with v-if rather than render `0` / Date(0).
export interface SearchResultUser extends KunUser {
  bio: string
  moemoepoint?: number
  created?: Date | string
}

export interface SearchResultReply {
  topic_id: number
  topic_title: string
  floor: number
  content: string
  user: KunUser
  created: Date | string
}

// `target_user` is the "comment-chain parent" — BE `CommentItem`
// doesn't carry it today; FE guards rendering with v-if.
export type SearchResultComment = {
  // Comment id → deep-link to it (/topic/:id?comment=<id>).
  id: number
  topic_id: number
  topic_title: string
  content: string
  user: KunUser
  target_user?: KunUser
  created: Date | string
}

export type SearchType = 'topic' | 'galgame' | 'user' | 'reply' | 'comment'
export type SearchResult =
  | SearchResultTopic
  | SearchResultGalgame
  | SearchResultUser
  | SearchResultReply
  | SearchResultComment
