export default defineNuxtRouteMiddleware((to) => {
  const { id } = usePersistUserStore()

  // Not logged in → the unified login prompt (mirrors the `auth` middleware),
  // carrying where they were headed.
  if (!id) {
    return navigateTo(
      `/auth/required?redirect=${encodeURIComponent(to.fullPath)}`
    )
  }

  // Logged in but lacking the site-administration capability (admin ⊂ ren):
  // this area is admin-only, so bounce non-admins home. This is a UX guard —
  // the real boundary is the API's RequireAdmin gate on the write routes.
  const { canAdminister } = useRole()
  if (!canAdminister.value) {
    return navigateTo('/')
  }
})
