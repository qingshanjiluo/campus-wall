// Pure helpers for the editkit family. No forum imports (extraction-ready
// boundary — see types.ts). `diff` is an npm dependency, not forum code, so it
// travels with the extraction.

import { diffWords } from 'diff'

import type { EditControl, EditFieldConfig, EditSchemaField } from './types'

/** Deterministic stringify (sorted object keys) so value identity survives
 * JSON round trips regardless of key order. */
export const stableStringify = (value: unknown): string => {
  if (value === null || value === undefined) {
    return 'null'
  }
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(',')}]`
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([k, v]) => `${JSON.stringify(k)}:${stableStringify(v)}`)
    return `{${entries.join(',')}}`
  }
  return JSON.stringify(value)
}

export const editValueEqual = (a: unknown, b: unknown): boolean =>
  stableStringify(a) === stableStringify(b)

/** Resolve the control a field renders with: host config wins, then the
 * schema's kind/diff_hint defaults. */
export const resolveControl = (
  field: EditSchemaField,
  config?: EditFieldConfig
): EditControl => {
  if (config?.control) {
    return config.control
  }
  switch (field.kind) {
    case 'text':
      return field.diff_hint === 'lines' ? 'textarea' : 'input'
    case 'enum':
      return config?.options?.length ? 'select' : 'input'
    case 'int':
    case 'ref':
      return 'number'
    case 'date':
      return 'date'
    case 'imagehash':
      return 'image'
    case 'list':
      return field.diff_hint === 'image' ? 'image-list' : 'string-list'
    default:
      return 'readonly'
  }
}

/** Human display for a scalar value (readonly rendering + inline diff). */
export const formatEditValue = (
  value: unknown,
  config?: EditFieldConfig
): string => {
  if (config?.formatValue) {
    return config.formatValue(value)
  }
  if (value === null || value === undefined || value === '') {
    return '（空）'
  }
  if (typeof value === 'boolean') {
    return value ? '是' : '否'
  }
  if (config?.options) {
    const hit = config.options.find((o) => o.value === value)
    if (hit) {
      return hit.label
    }
  }
  if (Array.isArray(value)) {
    return value.map((v) => formatEditItem(v, config)).join('、') || '（空）'
  }
  if (typeof value === 'object') {
    return JSON.stringify(value)
  }
  return String(value)
}

/** Human display for one list item. */
export const formatEditItem = (
  item: unknown,
  config?: EditFieldConfig
): string => {
  if (config?.formatItem) {
    return config.formatItem(item)
  }
  if (item === null || item === undefined) {
    return '（空）'
  }
  if (typeof item === 'object') {
    const o = item as Record<string, unknown>
    if (typeof o.name === 'string' && typeof o.link === 'string') {
      return `${o.name} → ${o.link}`
    }
    return JSON.stringify(item)
  }
  return String(item)
}

// ─── Word-level text diff ─────────────────────────────
//
// Backed by jsdiff (npm `diff`, BSD-3) rather than a hand-rolled LCS: it is the
// de-facto standard, already coalesces runs, and is the engine the GitHub-style
// viewers build on. It replaced two separate in-house implementations here — a
// line-level LCS for text fields and a char-level one for the taxonomy snapshot
// columns — neither of which could answer "what changed" for prose.
//
// The presentation this feeds is a UNIFIED inline diff: one block per field
// with only the changed runs tinted. Two columns printing the whole field twice
// hide a one-character edit inside a 400-character intro. Word-level
// highlighting is the industry norm for exactly this (line-level for code,
// intra-line word highlighting for text).

export type TextDiffOp = 'equal' | 'insert' | 'delete'

export interface TextDiffSegment {
  op: TextDiffOp
  text: string
}

// CJK word segmentation.
//
// jsdiff's default tokenizer is regex-based and splits on whitespace, so for
// Chinese / Japanese — which have none — it falls back to per-character
// boundaries. That still finds the change, but it cuts words in half:
//   我喜[-欢][+爱]这个游戏      (no segmenter)
//   我[-喜欢][+喜爱]这个游戏    (zh segmenter)
// The second is what a reader actually parses, so pass a segmenter. Upstream
// recommends this for languages without spaces.
//
// Built once and reused — constructing an Intl.Segmenter is not free, and this
// runs per field per revision. `zh` also segments Japanese kana runs sensibly,
// and this forum's content is overwhelmingly Chinese.
//
// The capability check is not defensiveness for its own sake: an absent
// Intl.Segmenter would throw inside a render function and blank the whole diff,
// and losing word alignment is a far better outcome than that.
let segmenter: Intl.Segmenter | null | undefined

const wordSegmenter = (): Intl.Segmenter | null => {
  if (segmenter !== undefined) {
    return segmenter
  }
  segmenter =
    typeof Intl !== 'undefined' && typeof Intl.Segmenter === 'function'
      ? new Intl.Segmenter('zh', { granularity: 'word' })
      : null
  return segmenter
}

/** Turn a before/after pair into the runs to render. Identical inputs collapse
 * to a single `equal` run (or nothing when both are empty), so callers can
 * treat "no visible change" uniformly. */
export const diffTextSegments = (
  before: string,
  after: string
): TextDiffSegment[] => {
  const a = before ?? ''
  const b = after ?? ''
  if (!a && !b) {
    return []
  }
  if (a === b) {
    return [{ op: 'equal', text: a }]
  }

  const seg = wordSegmenter()
  const parts = diffWords(a, b, seg ? { intlSegmenter: seg } : undefined)

  return parts.map((p) => ({
    op: p.added ? 'insert' : p.removed ? 'delete' : 'equal',
    text: p.value
  }))
}

/** Count CHARACTERS added / removed, which is what the reader is judging ("a
 * word changed" vs "half the intro was rewritten"). Token counts would be less
 * legible for CJK, where one token is often one character. */
export const diffTextStats = (
  segments: TextDiffSegment[]
): { added: number; removed: number } => {
  let added = 0
  let removed = 0
  for (const s of segments) {
    if (s.op === 'insert') {
      added += s.text.length
    } else if (s.op === 'delete') {
      removed += s.text.length
    }
  }
  return { added, removed }
}

// Long untouched runs are folded the way a code review folds context: the
// reader came for the change, not the 250 characters around it.
export const TEXT_DIFF_ELIDE_OVER = 180
const TEXT_DIFF_CONTEXT = 60

export type TextDiffPiece =
  | { kind: 'text'; op: TextDiffOp; text: string }
  | { kind: 'elision'; count: number }

/** True when folding would actually hide something, i.e. the toggle is worth
 * offering. A single `equal` run means "no change at all" — never elide it, or
 * an unchanged field renders as nothing but a fold marker. */
export const isTextDiffElidable = (segments: TextDiffSegment[]): boolean =>
  segments.length > 1 &&
  segments.some((s) => s.op === 'equal' && s.text.length > TEXT_DIFF_ELIDE_OVER)

/** Expand the segments into renderable pieces, folding long unchanged runs
 * unless `expanded`. */
export const elideTextDiff = (
  segments: TextDiffSegment[],
  expanded: boolean
): TextDiffPiece[] => {
  if (expanded || segments.length === 1) {
    return segments.map((s) => ({ kind: 'text', op: s.op, text: s.text }))
  }
  const out: TextDiffPiece[] = []
  segments.forEach((s, i) => {
    if (s.op !== 'equal' || s.text.length <= TEXT_DIFF_ELIDE_OVER) {
      out.push({ kind: 'text', op: s.op, text: s.text })
      return
    }
    // Keep the context on the side that faces a change: the run before the
    // first change only needs its tail, the run after the last only its head.
    const c = TEXT_DIFF_CONTEXT
    if (i === 0) {
      out.push({ kind: 'elision', count: s.text.length - c })
      out.push({ kind: 'text', op: 'equal', text: s.text.slice(-c) })
    } else if (i === segments.length - 1) {
      out.push({ kind: 'text', op: 'equal', text: s.text.slice(0, c) })
      out.push({ kind: 'elision', count: s.text.length - c })
    } else {
      out.push({ kind: 'text', op: 'equal', text: s.text.slice(0, c) })
      out.push({ kind: 'elision', count: s.text.length - 2 * c })
      out.push({ kind: 'text', op: 'equal', text: s.text.slice(-c) })
    }
  })
  return out
}

export interface ItemsDiff {
  added: unknown[]
  removed: unknown[]
  kept: unknown[]
}

/** Item-level diff for list fields (diff_hint=items/image): identity is the
 * stable stringification. */
export const diffItems = (from: unknown, to: unknown): ItemsDiff => {
  const a = Array.isArray(from) ? from : []
  const b = Array.isArray(to) ? to : []
  const aKeys = new Set(a.map(stableStringify))
  const bKeys = new Set(b.map(stableStringify))
  return {
    added: b.filter((x) => !aKeys.has(stableStringify(x))),
    removed: a.filter((x) => !bKeys.has(stableStringify(x))),
    kept: a.filter((x) => bKeys.has(stableStringify(x)))
  }
}

/** Status → KunUI chip color + label. */
export const proposalStatusBadge = (
  status: string
): { label: string; color: 'primary' | 'success' | 'danger' | 'default' } => {
  switch (status) {
    case 'open':
      return { label: '待审核', color: 'primary' }
    case 'merged':
      return { label: '已合并', color: 'success' }
    case 'declined':
      return { label: '已拒绝', color: 'danger' }
    case 'withdrawn':
      return { label: '已撤回', color: 'default' }
    default:
      return { label: status, color: 'default' }
  }
}

/** Engine action → badge label + color. Hosts may override labels via the
 * RevisionTimeline prop. */
export const revisionActionBadge = (
  action: string
): { label: string; color: 'primary' | 'success' | 'warning' | 'default' } => {
  switch (action) {
    case 'created':
      return { label: '创建', color: 'default' }
    case 'merged':
      return { label: '合并提案', color: 'success' }
    case 'direct':
      return { label: '直接编辑', color: 'primary' }
    case 'reverted':
      return { label: '回滚', color: 'warning' }
    default:
      return { label: action, color: 'default' }
  }
}
