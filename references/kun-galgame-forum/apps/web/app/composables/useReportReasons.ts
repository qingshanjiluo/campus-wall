// Module-cached report-reason catalog (GET /report/reasons). Fetched once and
// shared across every <ReportButton> on the page so N buttons don't each hit
// the endpoint.
const reasons = ref<ReportReason[]>([])
const loaded = ref(false)

export const useReportReasons = () => {
  const load = async () => {
    if (loaded.value) {
      return
    }
    const result = await kunFetch<ReportReason[]>('/report/reasons')
    if (result) {
      reasons.value = result
      loaded.value = true
    }
  }
  return { reasons, load }
}
