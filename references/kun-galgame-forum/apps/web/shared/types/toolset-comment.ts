export interface ToolsetComment {
  id: number
  toolset_id: number
  created: Date | string
  content: string
  edited: Date | string | null
  parent_id: number | null
  user_id: number
  // Roots carry their full set of descendants flattened here (oldest-first, one
  // visual tier); replies leave it empty. replyCount = reply.length (all loaded;
  // "展开更多" reveals them inline, no lazy fetch).
  reply: ToolsetComment[]
  reply_count: number
  user: KunUser
  target_user?: KunUser | null
}
