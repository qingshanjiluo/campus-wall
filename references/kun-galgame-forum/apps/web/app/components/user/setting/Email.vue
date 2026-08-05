<script setup lang="ts">
// Email change moved out of kungal entirely (docs/oauth/02-user-profile
// .md: "修改 email 不在这里 —— email 必须走 /auth/email/send-code
// + /auth/email"). The old in-app two-step flow is gone; users jump
// to the OAuth account-center profile page, which owns the
// send-code + confirm pipeline. Once they finish there, the
// usual session-refresh cycle picks up the new email at next sign-in.
//
// Account-center web app root + /profile. Read DIRECTLY from
// `oauthFrontendUrl` runtime config — do NOT try to derive it from
// `oauthServerUrl`. Dev runs the API on :9277 but the FE on :9420
// (different ports), so the old "strip /api/vN/" derivation would
// land the user on the wrong port.
const oauthProfileURL = computed(
  () => `${useRuntimeConfig().public.oauthFrontendUrl}/profile`
)
</script>

<template>
  <KunCard :is-hoverable="false" content-class="space-y-3">
    <div class="space-y-2">
      <span class="text-xl">更改邮箱</span>
      <p class="text-default-500 text-sm">
        邮箱由 鲲 Galgame OAuth 账户中心统一管理。修改邮箱需要二次验证 (邮箱验证码),
        请前往 鲲 Galgame OAuth 账户中心进行修改。修改后下次刷新页面即会同步至
        {{ kungal.titleShort }}。
      </p>
    </div>

    <div class="flex justify-end">
      <KunButton :href="oauthProfileURL" target="_blank">
        前往 鲲 Galgame OAuth 账户中心
      </KunButton>
    </div>
  </KunCard>
</template>
