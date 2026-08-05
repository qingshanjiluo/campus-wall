<script setup lang="ts">
import {
  KUN_PERMISSION_KEYS,
  KUN_PERMISSION_META,
  KUN_PERM_EDITABLE_ROLES,
  KUN_PERM_ROLE_COLUMNS
} from '~/constants/permission'

// Admin-only (admin ⊂ ren): mirrors the API's RequireAdmin gate on the
// /admin/role-permissions routes. UX guard — the API is the real boundary.
definePageMeta({ middleware: 'admin' })
useKunDisableSeo('权限管理')

// Two views: the role matrix (the Phase 2 surface, unchanged) and the change
// log. Plain local tab state — no ?tab= deep-link needed.
const permTabs = [
  { value: 'matrix', textValue: '权限矩阵' },
  { value: 'audit', textValue: '变更日志' }
]
const activeTab = ref('matrix')

const { data, refresh } = await useKunFetch<KunRolePermMatrix>(
  '/admin/role-permissions'
)

// A locally-mutable copy of the matrix; replaced wholesale after every save so
// the baseline reference stays authoritative.
const matrix = ref<KunRolePermMatrix | null>(data.value ?? null)

// WORKING effective sets for the three editable roles (creator/moderator/admin).
// Rebuilt from a matrix on load, discard and after each save. Toggling replaces
// the whole record (immutable) so Vue tracks the change without reactive Sets.
const buildWorking = (m: KunRolePermMatrix): Record<string, Set<string>> =>
  Object.fromEntries(
    KUN_PERM_EDITABLE_ROLES.map(
      (role) =>
        [role, new Set(m.roles[role]?.effective ?? [])] as [string, Set<string>]
    )
  )

const working = ref<Record<string, Set<string>>>(
  matrix.value ? buildWorking(matrix.value) : {}
)

// Baseline + effective sets for EVERY column (ren included, for its locked
// display), read straight off the current matrix.
const baseline = computed<Record<string, Set<string>>>(() =>
  Object.fromEntries(
    KUN_PERM_ROLE_COLUMNS.map(
      (col) =>
        [col.role, new Set(matrix.value?.roles[col.role]?.baseline ?? [])] as [
          string,
          Set<string>
        ]
    )
  )
)
const effective = computed<Record<string, Set<string>>>(() =>
  Object.fromEntries(
    KUN_PERM_ROLE_COLUMNS.map(
      (col) =>
        [col.role, new Set(matrix.value?.roles[col.role]?.effective ?? [])] as [
          string,
          Set<string>
        ]
    )
  )
)

const toggle = (role: string, permission: string, value: boolean) => {
  const next = new Set(working.value[role])
  if (value) {
    next.add(permission)
  } else {
    next.delete(permission)
  }
  working.value = { ...working.value, [role]: next }
}

// The DELTA vs baseline for one role: an override row per key where working
// diverges from baseline. Computed over the 43 keys only, so a no-op (granting
// what the baseline has / revoking what it lacks) can never be sent.
const deltasFor = (role: string) => {
  const base = baseline.value[role] ?? new Set<string>()
  const work = working.value[role] ?? new Set<string>()
  const out: { permission: string; effect: 'grant' | 'revoke' }[] = []
  for (const key of KUN_PERMISSION_KEYS) {
    const inWork = work.has(key)
    const inBase = base.has(key)
    if (inWork && !inBase) {
      out.push({ permission: key, effect: 'grant' })
    } else if (!inWork && inBase) {
      out.push({ permission: key, effect: 'revoke' })
    }
  }
  return out
}

const pendingByRole = computed(() =>
  Object.fromEntries(
    KUN_PERM_EDITABLE_ROLES.map(
      (role) => [role, deltasFor(role).length] as [string, number]
    )
  )
)
const dirtyRoles = computed(() =>
  KUN_PERM_EDITABLE_ROLES.filter((role) => (pendingByRole.value[role] ?? 0) > 0)
)
const totalPending = computed(() =>
  dirtyRoles.value.reduce(
    (sum, role) => sum + (pendingByRole.value[role] ?? 0),
    0
  )
)

// Containment guard: the backend rejects a moderator holding a permission admin
// lacks (moderator ⊆ admin must hold). Surface it inline BEFORE save.
const moderatorExceedsAdmin = computed(() => {
  const mod = working.value.moderator ?? new Set<string>()
  const admin = working.value.admin ?? new Set<string>()
  return KUN_PERMISSION_KEYS.filter(
    (key) => mod.has(key) && !admin.has(key)
  ).map((key) => KUN_PERMISSION_META[key].label)
})

const roleLabel = (role: string) =>
  KUN_PERM_ROLE_COLUMNS.find((col) => col.role === role)?.label ?? role

const applyMatrix = (m: KunRolePermMatrix) => {
  matrix.value = m
  working.value = buildWorking(m)
}

const handleDiscard = () => {
  if (matrix.value) {
    working.value = buildWorking(matrix.value)
  }
}

const saving = ref(false)
const handleSave = async () => {
  if (!totalPending.value || saving.value) {
    return
  }
  const ok = await useComponentMessageStore().alert(
    '保存权限调整',
    '此操作会立即改变所有持有相应角色的用户的实际权限，请确认无误后再保存。'
  )
  if (!ok) {
    return
  }
  saving.value = true
  let latest: KunRolePermMatrix | null = null
  let failed = false
  // One PUT per dirty role — each REPLACES that role's full override set and
  // returns the updated full matrix; the last success reflects every change.
  for (const role of dirtyRoles.value) {
    const res = await kunFetch<KunRolePermMatrix>(
      `/admin/role-permissions/${role}`,
      { method: 'PUT', body: { overrides: deltasFor(role) } }
    )
    if (res) {
      latest = res
    } else {
      failed = true
      break
    }
  }
  saving.value = false
  if (latest) {
    applyMatrix(latest)
  } else if (failed) {
    // Nothing persisted before the failure — resync from the server anyway.
    await refresh()
    if (data.value) {
      applyMatrix(data.value)
    }
  }
  if (!failed) {
    useMessage('已保存', 'success')
  }
}

const handleReset = async (role: string) => {
  const ok = await useComponentMessageStore().alert(
    `重置「${roleLabel(role)}」为默认`,
    '将清除该角色的全部覆盖，恢复到代码基线。此操作会立即生效，请确认。'
  )
  if (!ok) {
    return
  }
  const res = await kunFetch<KunRolePermMatrix>(
    `/admin/role-permissions/${role}`,
    { method: 'PUT', body: { overrides: [] } }
  )
  if (res) {
    applyMatrix(res)
    useMessage('已重置为默认', 'success')
  }
}
</script>

<template>
  <div class="w-full space-y-4">
    <div>
      <h1 class="text-2xl font-bold">权限管理</h1>
      <p class="text-default-500 text-sm">
        默认权限由代码基线定义；此处的调整是相对基线的授予 /
        撤销，重置即回到基线。前端仅为 UX，真正的边界始终在后端。
      </p>
    </div>

    <KunTab
      v-model="activeTab"
      :items="permTabs"
      variant="underlined"
      color="primary"
      size="sm"
    />

    <AdminPermissionAuditLog v-if="activeTab === 'audit'" />

    <template v-else>
      <KunInfo
        color="info"
        title="调整说明"
        description="勾选 = 授予该角色一项高于基线的权限；取消勾选 = 撤销一项基线本有的权限。带有小圆点的单元格即为相对基线的覆盖（绿色=授予，黄色=撤销）。莲（ren）恒持全部权限，不可调整。"
      />

      <KunInfo
        v-if="moderatorExceedsAdmin.length"
        color="warning"
        title="权限包含关系冲突"
        :description="`版主不应持有管理员没有的权限（后端会拒绝）：${moderatorExceedsAdmin.join('、')}。请为管理员一并授予，或撤销版主的该项。`"
      />

      <template v-if="matrix">
        <KunCard :is-hoverable="false" :is-transparent="false" padding="sm">
          <AdminPermissionMatrix
            :working="working"
            :baseline="baseline"
            :effective="effective"
            :disabled="saving"
            @toggle="toggle"
            @reset="handleReset"
          />
        </KunCard>

        <AdminPermissionProxyList />

        <!-- Sticky save bar: pin the surface to alpha 1 (bg-content1 carries the
           --kun-surface-opacity glass alpha; a lowered 透明度 setting would
           otherwise let the grid show through). -->
        <div class="sticky bottom-0 z-20 pb-3">
          <KunCard
            :is-hoverable="false"
            :is-transparent="false"
            class-name="bg-[oklch(var(--content1))]!"
          >
            <div class="flex flex-wrap items-center justify-between gap-3">
              <span class="text-default-500 text-sm">
                <template v-if="totalPending">
                  待保存
                  <span class="text-warning font-medium">{{
                    totalPending
                  }}</span>
                  项调整 ·
                  {{
                    dirtyRoles
                      .map(
                        (role) => `${roleLabel(role)} ${pendingByRole[role]}`
                      )
                      .join('，')
                  }}
                </template>
                <template v-else>尚无待保存的调整</template>
              </span>
              <div class="flex items-center gap-2">
                <KunButton
                  variant="light"
                  color="default"
                  :disabled="!totalPending || saving"
                  @click="handleDiscard"
                >
                  放弃更改
                </KunButton>
                <KunButton
                  color="primary"
                  :disabled="!totalPending"
                  :loading="saving"
                  @click="handleSave"
                >
                  <KunIcon name="lucide:check" />
                  保存
                </KunButton>
              </div>
            </div>
          </KunCard>
        </div>
      </template>

      <KunNull v-else description="无法加载权限矩阵，请稍后重试" />
    </template>
  </div>
</template>
