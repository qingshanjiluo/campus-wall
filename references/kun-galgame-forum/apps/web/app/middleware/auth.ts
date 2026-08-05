export default defineNuxtRouteMiddleware((to) => {
  const { id } = usePersistUserStore()

  // Send unauthenticated traffic to the unified login-prompt page (noindex),
  // carrying where they were headed so it can offer a one-click way back. We
  // don't auto-kick the OAuth flow from middleware (it runs in both SSR and
  // client; a forced window.location away would surprise users mid-navigation)
  // — the prompt page owns the explicit 登录 / 注册 buttons.
  if (!id) {
    return navigateTo(
      `/auth/required?redirect=${encodeURIComponent(to.fullPath)}`
    )
  }
})
