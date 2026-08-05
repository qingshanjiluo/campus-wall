// The wiki's own three-value axis. It is still the vocabulary the STAFF edit
// form writes (that face keeps its wiki rows until the editing engine retires
// it), so the type stays — but the public browse pages no longer render it:
// the canonical vocabulary that replaced the wiki tag table has no such axis
// (refs/proj/126 P2, the axis did not migrate).
export const KUN_GALGAME_TAG_TYPE = ['content', 'technical', 'sexual'] as const

export type KunGalgameTagCategory = (typeof KUN_GALGAME_TAG_TYPE)[number]

// Public tag categories after the catalog re-anchoring. `content` / `meta` are
// the canonical vocabulary's own kinds; `sexual` is reconstructed from the
// tag's own adult flag, the one distinction the SFW view acts on. It is NOT the
// hidden tier — that is an unrelated do-not-display axis (junk terms), and the
// backend read the two apart in the same wave the flag was populated.
export const KUN_GALGAME_TAG_CATEGORY_MAP: Record<string, string> = {
  content: '游戏内容',
  meta: '作品属性',
  sexual: '成人内容',
  technical: '技术细节'
}

export const KUN_GALGAME_TAG_SPOILER_TYPE = [0, 1, 2] as const

export type KunGalgameTagSpoiler = (typeof KUN_GALGAME_TAG_SPOILER_TYPE)[number]

export const KUN_GALGAME_TAG_SPOILER_MAP: Record<KunGalgameTagSpoiler, string> =
  {
    0: '无剧透',
    1: '轻微剧透',
    2: '严重剧透'
  }
