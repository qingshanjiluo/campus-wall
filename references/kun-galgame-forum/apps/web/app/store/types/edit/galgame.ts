export interface GalgameStorePersist {
  vndb_id: string
  name: KunLanguage
  introduction: KunLanguage
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  // Wiki-owned code (30+ values, e.g. ja-jp / ko-kr / ru), NOT one of the 4
  // content locales — must be a plain string so any wiki language round-trips.
  original_language: string
  aliases: string[]
  // U1: "" = unknown; sent as wire `release_date` (snake_case at boundary
  // by Footer). releaseDateTBA can coexist with a concrete date (semantic
  // "predicted Y/M").
  release_date: string
  release_date_tba: boolean
}

// PR-edit working copy (also reused by Draft.vue). Backs the temp,
// non-persisted store: it is fully re-hydrated from the galgame detail
// every time the user opens the rewrite form, so leftover state never
// bleeds across edits.
//
// tags/officials/engines hold the FULL entity objects (not just ids) so
// the selectors can render names without a second lookup; Footer.vue
// derives the *_ids[] wire arrays at submit time. links mirrors the wiki
// PR `links` shape ({name, link}). Two distinct description surfaces the
// wiki keeps separate (docs/galgame_wiki/02-revisions-and-prs.md): a
// direct edit writes a REVISION → uses `note`; opening a PR → uses
// `title` + `message`. Footer.vue sends whichever the submit endpoint
// consumes, based on can_direct_edit.
//
// IMPORTANT (wiki "replace-all" semantics): aliases / tag_ids /
// official_ids / engine_ids / links each REPLACE the entire set on
// merge. These must be pre-hydrated COMPLETELY from current data —
// see components/galgame/Rewrite.vue — or a submit silently wipes them.
export interface GalgameEditStoreTemp {
  id: number
  vndb_id: string
  name: KunLanguage
  introduction: KunLanguage
  content_limit: 'sfw' | 'nsfw'
  age_limit: 'all' | 'r18'
  // Wiki-owned code (30+ values, e.g. ja-jp / ko-kr / ru), NOT one of the 4
  // content locales — must be a plain string so any wiki language round-trips.
  original_language: string
  alias: string[]
  tags: GalgameTagItem[]
  officials: GalgameOfficialItem[]
  engines: GalgameEngineItem[]
  links: { name: string; link: string }[]
  // Revision note (direct-edit path). PR path uses title + message instead.
  note: string
  title: string
  message: string
  // U1: "" = unknown (cleared); concrete "YYYY-MM-DD" or any (TBA may set
  // a predicted partial date — wiki accepts the same string format).
  release_date: string
  release_date_tba: boolean
  // U2: cover candidate set + screenshot gallery, presence-replace
  // (wiki PUT/PR treats the whole array as authoritative). Pre-hydrated
  // COMPLETELY from galgame.covers/screenshots in Rewrite.vue — drop a
  // row by removing from the array, add new by uploading then pushing.
  // cdn_url is a server-injected derived field for previews; Footer
  // strips it before sending to wiki (wiki doesn't accept it on write).
  covers: GalgameCover[]
  screenshots: GalgameScreenshot[]
  // JSON snapshot of covers/screenshots AS HYDRATED. Footer compares the
  // current arrays against these to tell whether the user actually edited the
  // image set; if not, it OMITS covers/screenshots from the PR payload
  // (presence-replace: nil = keep). Without this, every intro/tag/title-only
  // edit re-sent the hydrated covers — and if that hydration was even slightly
  // stale (cached detail, or the cover changed elsewhere after the form
  // opened) the live cover silently rolled back. Optional: only the
  // rewrite/PR flow sets them (Draft's PATCH never touches images).
  covers_baseline?: string
  screenshots_baseline?: string
  // Same presence-replace guard for the (replace-all) links: Footer omits
  // `links` from the payload when it equals this baseline, so the wiki keeps
  // the current set. Critically, if the `/link/all` hydration FAILED, both
  // `links` and this baseline are empty → "unchanged" → omitted → existing
  // links are preserved instead of being wiped to nothing on merge.
  links_baseline?: string
  // True when the current user is the galgame's creator or an
  // admin/moderator (canModerate): wiki lets them edit directly via
  // PUT /galgame/:gid (instant, new revision) instead of opening a PR.
  // Computed once at hydration (Rewrite.vue has galgame.user + the user
  // store); Footer.vue branches the submit endpoint on it. Draft.vue
  // sets it false (drafts always PATCH, never this path).
  can_direct_edit: boolean
}
