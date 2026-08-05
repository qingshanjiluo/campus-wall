// PR list item (kungal maps /galgame/:gid/pr/all → camelCase).
// PRs carry title + message (the old single `note` was retired for PRs;
// revisions still use `note`). See docs/galgame_wiki/02-revisions-and-prs.md.
export interface GalgamePR {
  id: number
  galgame_id: number
  status: number
  title: string
  message: string
  base_revision: number
  user: KunUser
  completed_time: Date | string | null
  created: Date | string
}

// id → displayName lookup dicts shipped alongside the wiki diff/PR
// responses (K-PR 2026-Q2). Scoped to the ids referenced by THIS
// specific diff so the frontend can render entity names inline
// without an N+1 follow-up. Missing key ⇒ entity deleted ⇒ frontend
// falls back to "已删除 #<id>".
export interface NextMoeSnapshotNames {
  tags?: Record<string, string>
  officials?: Record<string, string>
  engines?: Record<string, string>
  series?: Record<string, string>
}

// Raw wiki PR detail response: GET /galgame/:gid/prs/:id (ProxyGet,
// passed through verbatim — snake_case). See docs 02-revisions-and-prs.
export interface NextMoePRDetailResponse {
  pr: {
    id: number
    galgame_id: number
    user_id: number
    status: number
    title: string
    message: string
    base_revision: number
    snapshot: Record<string, unknown>
    completed_by: number | null
    revision_id: number | null
    created: string
  }
  changed_keys: Record<string, boolean>
  names?: NextMoeSnapshotNames
}

// Normalized shape Info.vue builds and Details.vue renders: old =
// base-revision snapshot, new = pr.snapshot, limited to changed_keys.
export interface GalgamePRDiffView {
  id: number
  galgame_id: number
  status: number
  changed_keys: Record<string, boolean>
  old_snap: Record<string, unknown>
  new_snap: Record<string, unknown>
  names?: NextMoeSnapshotNames
}
