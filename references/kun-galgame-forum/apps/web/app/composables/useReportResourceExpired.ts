// Shared "report resource link expired" flow for the galgame resource UI.
//
// Since the backend gated expiry on an objective netdisk-API check
// (kungal-link-live-checker), one PUT does check → mark atomically and returns a
// { verdict, marked } body. This composable turns that into a small status
// machine so both call sites (resource detail Info + LinkDetailModal) can show a
// friendly check → mark flow instead of a bare spinner:
//
//   checking → the request is in flight (the netdisk check is the slow part)
//   expired  → verified dead (or unknown → legacy fallback) and marked
//   alive    → verified still reachable; NOT marked, NOT an error (good news)
//   error    → the request itself failed (kunFetch already toasted the reason)
export type ReportExpireStatus = 'idle' | 'checking' | 'expired' | 'alive' | 'error'

export const useReportResourceExpired = () => {
  const status = ref<ReportExpireStatus>('idle')
  // Captured during setup; the alert() await below drops the Nuxt instance
  // context, so kunFetch / the onMarked callback must be re-entered through it.
  const nuxtApp = useNuxtApp()

  const report = async (
    galgameId: number,
    resourceId: number,
    onMarked?: () => void
  ) => {
    if (!usePersistUserStore().id) {
      useAuthModal().open()
      return
    }

    const confirmed = await useComponentMessageStore().alert(
      '您确定报告资源链接失效吗？',
      '系统会先用网盘官方接口核验链接是否真的失效: 确认失效才会标记并通知发布者; 若链接仍可访问则不会标记。若 17 天内资源发布者没有更换有效链接, 该链接会被删除。恶意报告将被处罚。'
    )
    if (!confirmed) return

    status.value = 'checking'
    const result = await nuxtApp.runWithContext(() =>
      kunFetch<{ verdict: string; marked: boolean }>(
        `/galgame/${galgameId}/resource/expired`,
        { method: 'PUT', body: { galgame_resource_id: resourceId } }
      )
    )

    if (!result) {
      // kunFetch already surfaced the backend message (e.g. 已被标记为失效 /
      // network error); reflect a generic failure inline too.
      status.value = 'error'
      return
    }
    // Robust to the pre-deploy endpoint that returned a plain success message
    // string (no { marked }) — only an EXPLICIT alive verdict counts as "not
    // marked"; anything else (incl. the legacy string) means it was marked.
    const stillAlive = typeof result === 'object' && result.marked === false
    if (stillAlive) {
      status.value = 'alive'
    } else {
      status.value = 'expired'
      nuxtApp.runWithContext(() => onMarked?.())
    }
  }

  return { status, report }
}
