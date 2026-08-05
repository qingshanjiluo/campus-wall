export type UpdateType =
  | 'feat'
  | 'pref'
  | 'fix'
  | 'styles'
  | 'mod'
  | 'chore'
  | 'sec'
  | 'refactor'
  | 'docs'
  | 'test'

export interface UpdateTodo {
  id: number
  status: number
  type: string
  content_en_us: string
  content_ja_jp: string
  content_zh_cn: string
  content_zh_tw: string
  completed_time: Date | string | null
  user_id: number
  created: Date | string
  updated: Date | string
}

export interface UpdateLog {
  id: number
  type: UpdateType
  version: string
  content_en_us: string
  content_ja_jp: string
  content_zh_cn: string
  content_zh_tw: string
  user_id: number
  created: Date | string
  updated: Date | string
}

// Wire shapes of GET /update/history and GET /update/todo. The
// handlers each return `fiber.Map{"updates|todos": ..., "total":...}`
// so the outer key is endpoint-specific (not the shared `items`
// envelope). FE consumers previously typed these as undeclared
// `UpdateHistoryList` / `UpdateTodoList` — TS `any`.
export interface UpdateHistoryList {
  updates: UpdateLog[]
  total: number
}

export interface UpdateTodoList {
  todos: UpdateTodo[]
  total: number
}
