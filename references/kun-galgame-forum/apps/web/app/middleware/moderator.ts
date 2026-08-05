export default defineNuxtRouteMiddleware((to) => {
  const { id } = usePersistUserStore()

  // Not logged in → the unified login prompt (mirrors the `auth` middleware),
  // carrying where they were headed.
  if (!id) {
    return navigateTo(
      `/auth/required?redirect=${encodeURIComponent(to.fullPath)}`
    )
  }

  // Logged in but lacking the content-moderation capability (moderator ⊂ admin
  // ⊂ ren): this area is moderator-only, so bounce everyone else home. This is a
  // UX guard — the real boundary is the API's RequireModerator gate on the
  // routes these pages call.
  const { canModerate } = useRole()
  if (!canModerate.value) {
    return navigateTo('/')
  }
})
