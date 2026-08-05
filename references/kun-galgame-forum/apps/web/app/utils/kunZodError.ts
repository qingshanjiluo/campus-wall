// User-facing formatting for Zod (client-side) validation errors.
//
// The old format `位置: ${path[0]} - 错误提示: ${message}` leaked two things users
// couldn't read: the raw schema key as the "位置" (e.g. `short_summary`), and raw
// keys embedded in some hand-written messages. This maps a field key to a 中文
// label and renders one clean line — `简评：当总分为 1 或 10 分时需不少于 100 字`
// instead of `位置: short_summary - 错误提示: 当 overall 为 1 或 10 时, ...`.
//
// Default Zod messages (min/max/type/email…) are localized to 简体中文 globally by
// the zod-i18n plugin (z.config(z.locales.zhCN())); this only adds the field label
// in front so the user knows WHICH field the message is about.

// Raw schema field key → 用户能看懂的中文标签. Unmapped keys fall back to the raw
// key (no worse than before), so a new field degrades gracefully. Add entries as
// new user-facing forms appear.
export const KUN_FIELD_LABELS: Record<string, string> = {
  // ── galgame 评分 (galgame-rating) ──────────────────────────────
  overall: '总分',
  short_summary: '简评',
  galgameType: 'Galgame 类型',
  play_status: '游玩状态',
  spoiler_level: '剧透等级',
  recommend: '推荐度',
  art: '美术',
  story: '剧情',
  music: '音乐',
  character: '角色',
  route: '线路',
  system: '系统',
  voice: '配音',
  replay_value: '可重复游玩性',

  // ── galgame 词条 (galgame) ─────────────────────────────────────
  name_en_us: '英文名称',
  name_ja_jp: '日文名称',
  name_zh_cn: '简体中文名称',
  name_zh_tw: '繁体中文名称',
  intro_en_us: '英文简介',
  intro_ja_jp: '日文简介',
  intro_zh_cn: '简体中文简介',
  intro_zh_tw: '繁体中文简介',
  aliases: '别名',
  vndb_id: 'VNDB ID',
  release_date: '发售日期',
  original_language: '原始语言',
  age_limit: '年龄分级',
  content_limit: '内容分级',
  contentLimit: '内容分级',
  banner: '预览图',
  introduction: '简介',

  // ── 通用 ───────────────────────────────────────────────────────
  name: '名称',
  title: '标题',
  content: '内容',
  description: '描述',
  url: '链接',
  link: '链接',
  email: '邮箱',
  password: '密码',
  tags: '标签',
  targets: '标签',
  category: '分类',
  section: '分区',
  size: '大小',
  code: '验证码'
}

interface KunZodIssue {
  path?: (string | number)[]
  message: string
}

// First string segment of the issue path → its 中文 label (fallback: raw key).
const labelForPath = (path?: (string | number)[]): string => {
  const key = path?.find((p) => typeof p === 'string') as string | undefined
  if (!key) return ''
  return KUN_FIELD_LABELS[key] ?? key
}

// One parsed Zod issue → one clear, user-facing line. Leads with the field's
// 中文 label so the user knows WHERE; falls back to just the message when the
// issue has no field path (e.g. a root-level refine).
export const formatKunZodIssue = (issue: KunZodIssue): string => {
  const label = labelForPath(issue.path)
  return label ? `${label}：${issue.message}` : issue.message
}

// Convenience for a whole ZodError: format its first issue. Accepts anything
// with an `issues` array or the JSON-encoded `.message` the older call sites
// parse by hand.
export const formatKunZodError = (error: {
  issues?: KunZodIssue[]
  message?: string
}): string => {
  const issue =
    error.issues?.[0] ??
    (error.message ? (JSON.parse(error.message)[0] as KunZodIssue) : undefined)
  return issue ? formatKunZodIssue(issue) : '提交的内容有误，请检查后重试'
}
