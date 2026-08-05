<script setup lang="ts">
// Admin-only (admin ⊂ ren): the site-settings console sits alongside the other
// admin surfaces. UX guard — the real boundary is the API.
definePageMeta({ middleware: 'admin' })

useKunDisableSeo('网站设置')
</script>

<template>
  <div class="w-full space-y-6">
    <KunHeader name="网站设置" description="本论坛的全局配置项" />

    <KunCard :is-hoverable="false" :is-transparent="true">
      <p class="text-default-500 text-sm">
        网站注册开关等身份相关设置已迁移至 OAuth 管理后台, 此处暂无可配置项。
      </p>
    </KunCard>
  </div>
</template>
