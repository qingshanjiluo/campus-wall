// Whether any Galgame releases *today* (JST), for the desktop sidebar's
// 发售月历 indicator. Client-only (server: false) + keyed so it's fetched once
// per app load and never added to every page's SSR path. Reuses the cached
// /galgame/calendar month payload, which already carries `today`.
export const useGalgameReleaseToday = () => {
  const { data } = useKunFetch<GalgameCalendarMonth>('/galgame/calendar', {
    key: 'galgame-release-today',
    server: false
  })

  const hasReleaseToday = computed(
    () =>
      !!data.value?.items.some(
        (g) =>
          g.release_precision === 'day' && g.release_date === data.value!.today
      )
  )

  return { hasReleaseToday }
}
