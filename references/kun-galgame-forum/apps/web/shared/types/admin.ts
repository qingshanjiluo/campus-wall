export interface AdminOverStats {
  date: string
  [key: string]: number | string
}

// Per-type breakdown of a user's kungal content (BE: admin/dto
// UserContentStats). Used to preview and report a content purge.
export interface AdminUserContentStats {
  topics: number
  replies: number
  topic_comments: number
  ratings: number
  resources: number
  websites: number
  toolsets: number
  toolset_resources: number
  chat_messages: number
  messages: number
  interactions: number
  // Live community comment posts across all areas (galgame / rating / website /
  // toolset), backed by the community primitive.
  community_posts: number
  total: number
  // Purge-result only (DELETE /admin/user/:id/content): how many community
  // posts / reactions the purge actually removed. Absent on the preview.
  community_posts_purged?: number
  community_reactions_deleted?: number
}
