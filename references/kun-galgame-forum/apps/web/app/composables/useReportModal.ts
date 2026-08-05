// Shared state for the ONE global report modal (mounted at app.vue root). The
// modal must NOT live inside a <ReportButton> that sits in a ⋯ KunPopover: the
// popover's content is v-if'd, so clicking the (teleported, outside-the-popover)
// modal closes the popover and tears the modal down with no animation. So the
// button only triggers open(); the modal renders at the stable root. Mirrors
// the 萌萌点明细 modal pattern (KunTopBarMoemoepointLog in app.vue).
export interface ReportTarget {
  subjectKind: string
  subjectId: string | number
  snapshot?: string
  // Absolute deep-link to the content, so the moderator console opens it in
  // context (must be absolute — the link is clicked in infra's console).
  subjectUrl?: string
}

const isOpen = ref(false)
const target = ref<ReportTarget | null>(null)

export const useReportModal = () => {
  const open = (t: ReportTarget) => {
    target.value = t
    isOpen.value = true
  }
  return { isOpen, target, open }
}
