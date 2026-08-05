<script setup lang="ts">
// Unified "you must log in" landing page. The auth route middleware bounces
// every login-gated page here (instead of silently kicking the OAuth flow or
// dumping users on the homepage with a toast). noindex/nofollow — this is a
// transient gate, never something a crawler should surface.
useKunDisableSeo('需要登录')

// `redirect` carries where the user was headed so we can offer a one-click way
// back after they sign in (the OAuth round-trip itself still lands on /, but a
// logged-in user clicking this returns to their destination).
const route = useRoute()
const redirectTo = computed(() => {
  const r = (route.query.redirect as string) || ''
  // Only honour same-site absolute paths — never an attacker-supplied URL.
  return r.startsWith('/') && !r.startsWith('//') ? r : ''
})

const isJumping = ref(false)

const handleLogin = async () => {
  isJumping.value = true
  await startOAuthLogin()
}

const handleRegister = async () => {
  isJumping.value = true
  await startOAuthRegister()
}
</script>

<template>
  <div class="flex min-h-[calc(100dvh-12rem)] items-center justify-center p-4">
    <KunCard
      :is-transparent="false"
      :is-hoverable="false"
      class-name="w-full max-w-md"
      content-class="space-y-6"
    >
      <div class="flex flex-col items-center gap-3 text-center">
        <KunImage src="/favicon.webp" class-name="h-14 w-14 rounded-2xl" />
        <h1 class="text-xl font-bold">需要登录</h1>
        <p class="text-default-500 text-sm">
          该页面需要登录后才能访问。登录或注册以解锁完整功能，账号统一由
          <span class="text-default-700 font-medium">鲲 Galgame OAuth</span>
          管理。
        </p>
      </div>

      <div class="flex flex-col gap-3">
        <KunButton
          color="primary"
          size="lg"
          full-width
          :disabled="isJumping"
          @click="handleLogin"
        >
          {{ isJumping ? '跳转中...' : '登录' }}
        </KunButton>
        <KunButton
          variant="flat"
          color="primary"
          size="lg"
          full-width
          :disabled="isJumping"
          @click="handleRegister"
        >
          {{ isJumping ? '跳转中...' : '注册新账号' }}
        </KunButton>
      </div>

      <div class="flex items-center justify-center gap-4 text-sm">
        <KunLink :to="redirectTo || '/'" underline="hover" color="default">
          {{ redirectTo ? '我已登录，返回上一页' : '返回首页' }}
        </KunLink>
      </div>

      <p class="text-default-400 text-center text-xs">
        点击按钮将跳转至鲲 Galgame OAuth 统一认证系统
      </p>
    </KunCard>
  </div>
</template>
