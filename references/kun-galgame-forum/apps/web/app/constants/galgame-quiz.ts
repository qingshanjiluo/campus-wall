import type { KunUIColor } from '@kungal/ui-core'

// Spoiler scale, mirroring galgame-rating (none / portion / serious).
export const KUN_QUIZ_SPOILER_CONST = ['none', 'portion', 'serious'] as const

export const KUN_QUIZ_SPOILER_MAP: Record<string, string> = {
  none: '无剧透',
  portion: '部分剧透',
  serious: '严重剧透'
}

export const KUN_QUIZ_SPOILER_COLOR_MAP: Record<string, KunUIColor> = {
  none: 'success',
  portion: 'warning',
  serious: 'danger'
}

export const KUN_QUIZ_TYPE_CONST = [
  'single',
  'multiple',
  'judge',
  'fill',
  'essay'
] as const

// Types currently offered for authoring / filtering. fill & essay are
// temporarily disabled (the backend still supports all five, so re-enabling is
// just adding them back here).
export const KUN_QUIZ_ENABLED_TYPE_CONST = [
  'single',
  'multiple',
  'judge'
] as const

export const KUN_QUIZ_TYPE_MAP: Record<string, string> = {
  single: '单选',
  multiple: '多选',
  judge: '判断',
  fill: '填空',
  essay: '问答'
}

export const KUN_QUIZ_TYPE_ICON_MAP: Record<string, string> = {
  single: 'lucide:circle-dot',
  multiple: 'lucide:list-checks',
  judge: 'lucide:scale',
  fill: 'lucide:pencil-line',
  essay: 'lucide:text'
}

export const KUN_QUIZ_TYPE_COLOR_MAP: Record<string, KunUIColor> = {
  single: 'primary',
  multiple: 'secondary',
  judge: 'success',
  fill: 'warning',
  essay: 'default'
}

// The description shown next to each type in the 出题 form.
export const KUN_QUIZ_TYPE_DESCRIPTION_MAP: Record<string, string> = {
  single: '给出多个选项, 只有一个正确答案',
  multiple: '给出多个选项, 有一个或多个正确答案',
  judge: '判断一句话是否正确',
  fill: '填写一个或多个空, 系统忽略大小写与空格自动判分',
  essay: '开放式问答, 不自动判分、不发放萌萌点, 仅展示参考答案'
}

export const KUN_QUIZ_CATEGORY_CONST = [
  'plot',
  'character',
  'system',
  'music',
  'voice',
  'company',
  'trivia',
  'other'
] as const

export const KUN_QUIZ_CATEGORY_MAP: Record<string, string> = {
  plot: '剧情',
  character: '角色',
  system: '系统',
  music: '音乐',
  voice: '声优',
  company: '会社',
  trivia: '常识',
  other: '其他'
}

export const KUN_QUIZ_SORT_FIELD_CONST = [
  'update_time',
  'time',
  'view',
  'view_1d',
  'view_7d',
  'view_30d',
  'difficulty',
  'answer_count'
] as const

// Difficulty 1-10 → a short tier label + color. Tiers: 1-2 入门, 3-4 简单,
// 5-6 普通, 7-8 困难, 9-10 地狱.
export const kunQuizDifficultyLabel = (d: number): string => {
  if (d <= 2) return '入门'
  if (d <= 4) return '简单'
  if (d <= 6) return '普通'
  if (d <= 8) return '困难'
  return '地狱'
}

export const kunQuizDifficultyColor = (d: number): KunUIColor => {
  if (d <= 2) return 'success'
  if (d <= 4) return 'primary'
  if (d <= 6) return 'secondary'
  if (d <= 8) return 'warning'
  return 'danger'
}
