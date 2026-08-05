import type { GalgameCard } from './galgame'

export interface UserInfo {
  id: number
  name: string
  avatar: string
  // OAuth named-role set (empty for a plain user; `user` never appears). Drives
  // the profile badge via managementRoleLabel + a creator chip. `creator` is
  // orthogonal — no moderation/admin power.
  roles: string[]
  status: number
  moemoepoint: number
  bio: string
  created: Date | string

  upvote: number
  like: number
  dislike: number

  reply_created: number
  comment_created: number
  topic: number
  topic_poll: number

  galgame: number
  contribute_galgame: number
  galgame_comment: number
  galgame_rating: number

  galgame_resource: number
  galgame_toolset: number
  galgame_toolset_resource: number

  daily_topic_count: number
  daily_galgame_count: number
}

export interface UserTopic {
  id: number
  title: string
  created: Date | string
}

export type UserGalgame = GalgameCard

export interface UserGalgameResource {
  id: number
  galgame_id: number
  galgame_name: KunLanguage
  type: string
  language: string
  platform: string
  size: string
  link: string[]
  code: string
  password: string
  note: string
  status: number
  created: Date | string
}

export interface UserReply {
  topic_id: number
  // Floor → deep-link to this reply (/topic/:id?reply=<floor>).
  floor: number
  content: string
  created: Date | string
}

export interface UserGetUserReplyRequestData {
  id: number
  rid_array: number[]
}

export interface UserComment {
  // Comment id → deep-link to it (/topic/:id?comment=<id>).
  id: number
  topic_id: number
  content: string
  created: Date | string
}

export interface UserGetUserCommentRequestData {
  id: number
  cid_array: number[]
}

export interface UserFloatingCard extends KunUser {
  moemoepoint: number
  topic_count: number
  topic_reply_count: number
  topic_comment_count: number
  galgame_resource_count: number
}

export type UserUpdateAvatarResponseData = {
  avatar: string
  avatar_min: string
}

export type UserGetUserEmailResponseData = {
  email: string
}
