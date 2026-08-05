import {
  KUN_QUIZ_CATEGORY_CONST,
  KUN_QUIZ_CATEGORY_MAP,
  KUN_QUIZ_TYPE_CONST,
  KUN_QUIZ_ENABLED_TYPE_CONST,
  KUN_QUIZ_TYPE_MAP,
  kunQuizDifficultyLabel
} from '~/constants/galgame-quiz'

export const quizCategoryOptions = [
  { value: 'all', label: '全部分类' },
  ...KUN_QUIZ_CATEGORY_CONST.map((c) => ({
    value: c,
    label: KUN_QUIZ_CATEGORY_MAP[c] || ''
  }))
]

export const quizTypeOptions = [
  { value: 'all', label: '全部题型', disabled: false },
  ...KUN_QUIZ_TYPE_CONST.map((t) => {
    const enabled = (KUN_QUIZ_ENABLED_TYPE_CONST as readonly string[]).includes(
      t
    )
    return {
      value: t,
      label: enabled
        ? KUN_QUIZ_TYPE_MAP[t] || ''
        : `${KUN_QUIZ_TYPE_MAP[t] || ''}（即将实装）`,
      disabled: !enabled
    }
  })
]

// value 0 = "all" (the Go DTO drops difficulty=0 via omitempty → no filter).
export const quizDifficultyOptions = [
  { value: 0, label: '全部难度' },
  ...Array.from({ length: 10 }, (_, i) => i + 1).map((n) => ({
    value: n,
    label: `${n} · ${kunQuizDifficultyLabel(n)}`
  }))
]

export const quizSortFieldOptions = [
  { value: 'update_time', label: '最近更新' },
  { value: 'time', label: '创建时间' },
  { value: 'view_1d', label: '日浏览数' },
  { value: 'view_7d', label: '周浏览数' },
  { value: 'view_30d', label: '月浏览数' },
  { value: 'view', label: '总浏览数' },
  { value: 'difficulty', label: '难度' },
  { value: 'answer_count', label: '作答数' }
]
