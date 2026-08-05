// Trust & Safety moderation-inbox shapes, mirroring the infra trust admin API
// (proxied by kungal's BFF at /admin/trust/*). Integer enums are stable
// persisted codes (see constants/trust.ts for labels).

export interface ReviewItemView {
  id: number
  site: string
  subject_kind: string
  subject_id: string
  source: number // 0 reports … 5 manual
  severity?: number
  classifier_score?: number
  report_weight_sum?: number
  priority: number
  status: number // 0 pending / 1 claimed / 2 actioned / 3 dismissed
  claimed_by?: number
  claimed_at?: string
  decided_by?: number
  decided_at?: string
  created_at: string
}

export interface ReportView {
  id: number
  site: string
  subject_kind: string
  subject_id: string
  reporter_id: number
  reason_id: number
  note?: string
  subject_snapshot?: string
  // Absolute deep-link the reporter's page carried (trust 53d1f22).
  subject_url?: string
  weight: number
  review_item_id?: number
  status: number
  created_at: string
}

export interface ReviewItemDetail {
  item: ReviewItemView
  reports: ReportView[]
}

export interface ReviewItemPage {
  items: ReviewItemView[]
  total: number
}
