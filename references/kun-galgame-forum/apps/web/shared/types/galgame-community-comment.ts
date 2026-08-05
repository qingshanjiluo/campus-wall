// Wire shapes for the community-primitive-backed galgame comment section
// (charter step 03/04). These mirror the Go DTOs in
// apps/api/internal/galgame/service/community_comment_service.go and are ONLY
// consumed by the flag-gated `Community*` comment components — the legacy
// `GalgameComment` shape and its components are left untouched.
//
// The list is FLAT: each item carries root/reply pointers and the frontend does
// the two-tier grouping (root + one flat reply group, never deeper).

export interface GalgameCommunityComment {
  // Community post id (used for the anchor id and all post-addressed writes).
  id: number
  // Raw markdown source — round-trips through the edit-mode editor. Blank on a
  // tombstoned (deleted) post; the server never serves a deleted body.
  content: string
  // HTML rendered by kungal's own markdown pipeline (charter ruling 7). Drop
  // into <KunContent>; DOMPurify there is the final XSS guard.
  content_html: string
  galgame_id: number
  user: KunUser
  // reply_to_post_id → the direct parent; null on a root post.
  parent_comment_id: number | null
  // root_post_id → the thread's root; null on a root post. Drives grouping.
  root_comment_id: number | null
  // The @-replied user, when the post is a reply (omitted otherwise).
  target_user?: KunUser | null
  like_count: number
  is_liked: boolean
  // ISO timestamp.
  created: string
  // Author-driven edit timestamp; null = never edited. Drives the "已编辑" badge.
  edited: string | null
  // A moderator rewrote the body → the "已编辑（管理）" badge instead.
  edited_by_moderator: boolean
  // Community post status (0 visible / 1 held / 2 deleted); the two booleans
  // below are the decoded convenience flags.
  status: number
  // Tombstoned → render the "[已删除]" placeholder, keep the floor.
  deleted: boolean
  // TL0 first-post hold → author-only "审核中" badge (the server already hides
  // a held post from everyone but its author).
  held: boolean
}

// A flat keyset page of a comment area. next_cursor is an opaque post_number
// sentinel ("" = last page); total is the thread's posts_count.
export interface GalgameCommunityCommentPage {
  thread_id: number
  posts: GalgameCommunityComment[]
  next_cursor: string
  total: number
  // The area is withheld from THIS viewer behind a spoiler gate — currently only
  // a concealing quiz they have not answered (api
  // service/resource_comment_gate.go). The page then carries no posts and no
  // counts. Always false for every other area. The gate is decided server-side
  // BECAUSE the list is a public GET; the client only renders the ruling.
  locked: boolean
}

// The legacy-id → community-post resolution used for old deep-link continuity.
export interface GalgameCommunityCommentLocate {
  post_id: number
  thread_id: number
}
