// @mention links in rendered content are server-emitted plain <a> tags with an
// ABSOLUTE href (absolute so they survive the markdown sanitizer), so a normal
// click triggers a full-page reload. Intercept clicks on `.kun-mention` anywhere
// and route client-side via data-uid → /user/<id>/info (SPA, no reload). Modifier
// clicks (ctrl/cmd/shift/middle) fall through to the browser's open-in-new-tab.
export default defineNuxtPlugin(() => {
  const router = useRouter()

  document.addEventListener('click', (e) => {
    if (
      e.defaultPrevented ||
      e.button !== 0 ||
      e.metaKey ||
      e.ctrlKey ||
      e.shiftKey ||
      e.altKey
    ) {
      return
    }
    const mention = (e.target as HTMLElement | null)?.closest<HTMLElement>(
      'a.kun-mention'
    )
    const uid = mention?.dataset.uid
    if (!uid) {
      return
    }
    e.preventDefault()
    router.push(`/user/${uid}`)
  })
})
