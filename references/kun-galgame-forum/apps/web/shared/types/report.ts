// A report reason from GET /report/reasons (BFF-served, mirrors the infra
// Trust & Safety seeded global reasons). severity: 1=low … 3=high.
export interface ReportReason {
  key: string
  label: string
  severity: number
}
