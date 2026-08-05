// Wire types for the Galgame 题库 (quiz) feature. Field names are snake_case to
// match the Go DTOs verbatim (see apps/api/internal/galgame/dto/quiz_dto.go).

export type QuizType = 'single' | 'multiple' | 'judge' | 'fill' | 'essay'
export type QuizCategory =
  | 'plot'
  | 'character'
  | 'system'
  | 'music'
  | 'voice'
  | 'company'
  | 'trivia'
  | 'other'

export type QuizSpoilerLevel = 'none' | 'portion' | 'serious'

// The viewer's own status on a quiz (drives the list's left indicator).
export type QuizMyStatus =
  | 'author'
  | 'correct'
  | 'incorrect'
  | 'answered'
  | 'unanswered'

export interface QuizGalgameBrief {
  id: number
  content_limit: string
  name: KunLanguage
}

// Richer linked-game panel on the answer page's 查看详情.
export interface QuizGalgameDetail {
  id: number
  name: KunLanguage
  content_limit: string
  age_limit: string
  original_language: string
  banner: string
  banner_thumbhash?: string
  officials: string[]
}

export interface QuizStats {
  view: number
  answer_count: number
  correct_count: number
  favorite_count: number
  quality_average: number
  quality_count: number
  // Discussion counter (migration 065), maintained ±1 by the community comment
  // BFF — a tolerated display counter, not a source of truth.
  comment_count: number
}

export interface GalgameQuizCard extends QuizStats {
  id: number
  user: KunUser
  category: QuizCategory
  spoiler_level: QuizSpoilerLevel
  type: QuizType
  difficulty: number
  question: string
  created: string | Date
  updated: string | Date
  // Last-activity time (bumped on answer / author edit) — the list's 最近活跃 sort.
  status_update_time: string | Date
  my_status: QuizMyStatus
}

export interface QuizListPage {
  quiz_data: GalgameQuizCard[]
  total: number
}

// ── content payloads ──────────────────────────────────────────────
// The PUBLIC (pre-answer) payload keeps only what's needed to answer — the
// correct-answer key is stripped server-side.
export interface QuizPublicSingle {
  options: string[]
}
export interface QuizPublicMultiple {
  options: string[]
}
export interface QuizPublicFill {
  blank_count: number
}
export type QuizPublicContent =
  | QuizPublicSingle
  | QuizPublicMultiple
  | QuizPublicFill
  | Record<string, never>

// The FULL payload (revealed only after answering / to the author).
export interface QuizFullSingle {
  options: string[]
  answer: number
}
export interface QuizFullMultiple {
  options: string[]
  answers: number[]
}
export interface QuizFullJudge {
  answer: boolean
}
export interface QuizFullFillBlank {
  accepted: string[]
}
export interface QuizFullFill {
  blanks: QuizFullFillBlank[]
}
export interface QuizFullEssay {
  reference: string
}
export type QuizFullContent =
  | QuizFullSingle
  | QuizFullMultiple
  | QuizFullJudge
  | QuizFullFill
  | QuizFullEssay

// ── submitted answer payloads ─────────────────────────────────────
export type QuizSubmitted =
  | { value: number } // single
  | { values: number[] } // multiple
  | { value: boolean } // judge
  | { values: string[] } // fill
  | { text: string } // essay

export interface QuizAnswerResult {
  submitted: QuizSubmitted | null
  is_correct: boolean | null
  answer: QuizFullContent
  explanation: string
  quality_rating: number | null
  reward_delta: number
}

export interface GalgameQuizPlay extends QuizStats {
  id: number
  user: KunUser
  category: QuizCategory
  spoiler_level: QuizSpoilerLevel
  type: QuizType
  difficulty: number
  question: string
  content: QuizPublicContent
  // Server-rendered markdown (may embed image hints for "guess the game").
  description_html: string
  created: string | Date
  updated: string | Date
  hide_galgame: boolean
  // Empty while hide_galgame is on and the viewer hasn't answered yet.
  galgames: QuizGalgameDetail[]
  is_author: boolean
  is_favorited: boolean
  // Present once the viewer has answered (or is the author) — reveals the key.
  my_answer: QuizAnswerResult | null
}

export interface QuizQualityResult {
  quality_average: number
  quality_count: number
  quality_rating: number
}

// A galgame candidate returned by the 出题 picker search — enriched with a
// banner thumbnail + 会社 (maker) names.
export interface QuizGalgameOption {
  id: number
  name: KunLanguage
  banner: string
  banner_thumbhash?: string
  officials: string[]
}

// GET /galgame-quiz/:id/edit — the FULL quiz (answer key included) for the edit
// form (author or moderator only).
export interface QuizEditData {
  id: number
  galgame_ids: number[]
  hide_galgame: boolean
  category: QuizCategory
  type: QuizType
  difficulty: number
  spoiler_level: QuizSpoilerLevel
  question: string
  description: string
  content: QuizFullContent
  explanation: string
  galgames: QuizGalgameBrief[]
}

// GET /galgame-quiz/:id/answers — one answerer's outcome (card 查看详情 panel).
export interface QuizAnswererRecord {
  user: KunUser
  is_correct: boolean | null
  // The answerer's own submission — present ONLY when the viewer has answered
  // (or authored) this quiz; the server gates it, since it reveals the answer.
  submitted?: QuizSubmitted
  created: string | Date
}
