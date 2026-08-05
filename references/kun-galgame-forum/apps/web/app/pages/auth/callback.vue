<script setup lang="ts">
import Cookies from 'js-cookie'

// No layout: a transient redirect page wants a clean full-screen loader, not
// the blank layout's top-bar + content column (whose width-less, h-full column
// collapsed this page's `size-full` into a cramped, off-looking box).
definePageMeta({ layout: false })

const route = useRoute()
const error = ref('')

// OAuth callback is a transient redirect-only page — never something a
// search engine should index.
useKunDisableSeo('OAuth 登录回调')

onMounted(async () => {
  const code = route.query.code as string
  const returnedState = route.query.state as string
  // Read + clear the per-attempt secrets from COOKIES (set in oauth-auth.ts).
  // See there for why sessionStorage was abandoned — it lost these across the
  // cross-origin redirect on Via / older mobile browsers → "State 不匹配".
  const savedState = Cookies.get('oauth_state')
  const codeVerifier = Cookies.get('oauth_code_verifier')

  // Clean up (must match the path the cookies were set with).
  Cookies.remove('oauth_state', { path: '/' })
  Cookies.remove('oauth_code_verifier', { path: '/' })

  if (!code) {
    error.value = '未收到授权码'
    redirectToLogin()
    return
  }

  if (returnedState !== savedState) {
    error.value = 'State 不匹配，可能存在安全风险'
    redirectToLogin()
    return
  }

  if (!codeVerifier) {
    error.value = 'PKCE 验证器丢失，请重新登录'
    redirectToLogin()
    return
  }

  // Matches BE dto.UserProfile exactly. Email is owned by OAuth and
  // NOT returned here — the frontend fetches it from OAuth's
  // /oauth/userinfo on demand (per the BE comment on UserProfile).
  const result = await kunFetch<{
    id: number
    sub: string
    name: string
    avatar: string
    roles: string[]
    moemoepoint: number
    bio: string
  }>('/auth/oauth/callback', {
    method: 'POST',
    body: { code, code_verifier: codeVerifier }
  })

  if (result) {
    const userStore = usePersistUserStore()
    userStore.setUserInfo({
      id: result.id,
      sub: result.sub,
      name: result.name,
      avatar: result.avatar,
      // withImageVariant picks the right separator per URL family:
      // image_service hash-addressed URLs get `_100`, legacy nitro
      // paths still on image.kungal.com get `-100`. The legacy avatar
      // bulk migration is pending; until it lands both coexist for
      // active users.
      avatarMin: result.avatar ? withImageVariant(result.avatar, '100') : '',
      moemoepoint: result.moemoepoint,
      roles: result.roles ?? [],
      isCheckIn: false,
      dailyToolsetUploadBytes: 0
    })

    // Cache this account for the switch menu — login AND switch both land here,
    // so the active account stays current. (docs/oauth/09-account-switching §3.6)
    useKnownAccounts().rememberUser({
      sub: result.sub,
      id: result.id,
      name: result.name,
      avatar: result.avatar,
      roles: result.roles
    })

    useKunLoliInfo(`登录成功! 欢迎来到 ${kungal.name}`)
    // A switch/add flow stashed where to return; land back there (origin-guarded),
    // else home.
    await navigateTo(consumeOAuthReturnTo() ?? '/')
  } else {
    error.value = '登录失败，请重试'
    redirectToLogin()
  }
})

// After a failed callback, drop the user back at the homepage. The
// top-bar 登录 button (KunAuthModal) is one click away — no point in
// keeping a /login page that itself just shows the same modal.
const redirectToLogin = () => {
  setTimeout(() => navigateTo('/'), 2000)
}
</script>

<template>
  <div
    class="bg-background flex min-h-dvh w-full flex-col items-center justify-center gap-5 px-6 text-center"
  >
    <template v-if="!error">
      <KunIcon
        class="text-primary size-10 animate-spin"
        name="lucide:loader-circle"
      />
      <div class="space-y-1">
        <p class="text-foreground text-lg font-medium">正在登录...</p>
        <p class="text-default-500 text-sm">正在验证您的身份，请稍候</p>
      </div>
    </template>
    <template v-else>
      <KunIcon class="text-danger size-10" name="lucide:circle-alert" />
      <div class="space-y-1">
        <p class="text-danger text-lg font-medium">{{ error }}</p>
        <p class="text-default-500 text-sm">即将返回首页…</p>
      </div>
    </template>
  </div>
</template>
