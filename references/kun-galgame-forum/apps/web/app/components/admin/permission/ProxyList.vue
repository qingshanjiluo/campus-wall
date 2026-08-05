<script setup lang="ts">
import { KUN_PROXY_PERMISSIONS } from '~/constants/permission'

// Read-only surface for the 9 INFRA-PROXY operations. Their truth lives in
// kun-galgame-infra (editing engine / kun_trust), never in pkg/perm — so they
// are shown for completeness but cannot be overridden here.
</script>

<template>
  <KunCard
    :is-hoverable="false"
    :is-transparent="false"
    content-class="space-y-3"
  >
    <div>
      <h2 class="text-lg font-semibold">NextMoe（未萌）代理权限（只读）</h2>
      <p class="text-default-500 text-sm">
        以下操作的真闸在 NextMoe（未萌），论坛不可在此调整；taxonomy
        为莲裁定的更严门槛。修改需要在 NextMoe（未萌）侧进行。
      </p>
    </div>

    <div class="space-y-2">
      <div
        v-for="item in KUN_PROXY_PERMISSIONS"
        :key="item.key"
        class="border-default-100 flex flex-wrap items-center justify-between gap-2 rounded-lg border px-3 py-2"
      >
        <div class="flex items-center gap-2">
          <KunIcon name="lucide:external-link" class="text-default-400" />
          <span class="text-sm font-medium">{{ item.label }}</span>
        </div>
        <span class="text-default-500 text-xs">{{ item.note }}</span>
      </div>
    </div>

    <KunInfo
      color="info"
      description="这些权限由 NextMoe（未萌）强制执行，此处仅供总览。"
    />
  </KunCard>
</template>
