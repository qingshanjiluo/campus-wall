<script setup lang="ts">
import { storeToRefs } from 'pinia'
import {
  KUN_PERMISSION_GROUPS,
  KUN_PERMISSION_KEYS,
  KUN_PERMISSION_META,
  kunRoleRank
} from '~/constants/permission'
import { KUN_USER_ROLE_MAP } from '~/constants/user'

// Per-user permission overrides. Opened from the admin user page: an admin
// grants / revokes individual permissions ON TOP of what the user's roles give
// them. The deviation reference is `role_effective` (what the roles alone would
// grant); the PUT sends the working set's deltas vs that reference as the user's
// full personal override set (replace semantics). Self-contained — the caller
// only toggles `open` and passes the target user.
const props = defineProps<{ user: SearchResultUser }>()
const open = defineModel<boolean>({ required: true })

const state = ref<KunUserPermState | null>(null)
const loading = ref(false)
const saving = ref(false)

// WORKING effective set (local pending edits). Replaced wholesale on every
// toggle (immutable) so Vue tracks it without a reactive Set. `initial` is the
// loaded effective, the dirtiness reference.
const working = ref<Set<string>>(new Set())
const initial = ref<Set<string>>(new Set())

// The ren (莲) safety anchor holds the full catalogue and the backend refuses
// any override on it — render the whole panel read-only.
const isRen = computed(() => state.value?.roles.includes('ren') ?? false)

// Delegation guard (UX mirror of apps/api — the backend is the real boundary):
// an operator may only adjust a user of strictly-lower rank, and only toggle keys
// the operator themselves hold (possession). See constants/permission.ts + useCan.
const { roles: operatorRoles } = storeToRefs(usePersistUserStore())
const myPerms = useMyPermissions()
const targetRoles = computed(() => state.value?.roles ?? [])
const rankLocked = computed(
  () => kunRoleRank(operatorRoles.value) <= kunRoleRank(targetRoles.value)
)
// The whole panel is read-only for a ren target OR a same/higher-rank target.
const readOnly = computed(() => isRen.value || rankLocked.value)

// Cell-level possession lock: an editable-panel cell whose key the operator does
// not hold — visible (carried-over deviations still render) but not toggleable.
const possessionLocked = (permission: string) =>
  !readOnly.value && !myPerms.value(permission)
const cellDisabled = (permission: string) =>
  readOnly.value || saving.value || possessionLocked(permission)

// The role-derived reference set: the deviation baseline for the per-user panel.
const roleEffective = computed(() => new Set(state.value?.role_effective ?? []))

const groups = KUN_PERMISSION_GROUPS

const applyState = (s: KunUserPermState) => {
  state.value = s
  working.value = new Set(s.effective)
  initial.value = new Set(s.effective)
}

const load = async () => {
  loading.value = true
  const res = await kunFetch<KunUserPermState>(
    `/admin/user-permissions/${props.user.id}`
  )
  loading.value = false
  if (res) {
    applyState(res)
  }
}

// (Re)load each time the modal opens; clear stale state first so a reopen never
// flashes the previous user's grid.
watch(open, (isOpen) => {
  if (isOpen) {
    state.value = null
    working.value = new Set()
    initial.value = new Set()
    load()
  }
})

const toggle = (permission: string, value: boolean) => {
  const next = new Set(working.value)
  if (value) {
    next.add(permission)
  } else {
    next.delete(permission)
  }
  working.value = next
}

const isChecked = (permission: string) => working.value.has(permission)

// A working state diverging from role_effective is a personal override: 'grant'
// when the user now holds a permission their roles lack, 'revoke' when they lack
// one their roles grant.
const deviation = (permission: string): 'grant' | 'revoke' | null => {
  const inWork = working.value.has(permission)
  const inRole = roleEffective.value.has(permission)
  if (inWork === inRole) return null
  return inWork ? 'grant' : 'revoke'
}

// The full personal override set to PUT: one row per key where working diverges
// from role_effective. Computed over the 43 keys only, so a no-op relative to
// role_effective can never be sent (the backend rejects those too).
const deltas = computed(() => {
  const role = roleEffective.value
  const out: { permission: string; effect: 'grant' | 'revoke' }[] = []
  for (const key of KUN_PERMISSION_KEYS) {
    const inWork = working.value.has(key)
    const inRole = role.has(key)
    if (inWork && !inRole) {
      out.push({ permission: key, effect: 'grant' })
    } else if (!inWork && inRole) {
      out.push({ permission: key, effect: 'revoke' })
    }
  }
  return out
})

// Dirty = the working set differs from the loaded effective (what's persisted).
const isDirty = computed(() => {
  if (working.value.size !== initial.value.size) return true
  for (const key of working.value) {
    if (!initial.value.has(key)) return true
  }
  return false
})

const hasOverrides = computed(() => (state.value?.overrides.length ?? 0) > 0)

const roleChips = computed(() =>
  (state.value?.roles ?? []).map((role) => KUN_USER_ROLE_MAP[role] ?? role)
)

const handleDiscard = () => {
  working.value = new Set(initial.value)
}

const handleSave = async () => {
  if (!isDirty.value || saving.value || readOnly.value) {
    return
  }
  const ok = await useComponentMessageStore().alert(
    `调整「${props.user.name}」的权限`,
    '此操作会立即改变该用户的实际权限，请确认无误后再保存。'
  )
  if (!ok) {
    return
  }
  saving.value = true
  const res = await kunFetch<KunUserPermState>(
    `/admin/user-permissions/${props.user.id}`,
    { method: 'PUT', body: { overrides: deltas.value } }
  )
  saving.value = false
  if (res) {
    applyState(res)
    useMessage('已保存', 'success')
  }
}

const handleReset = async () => {
  if (!hasOverrides.value || saving.value || readOnly.value) {
    return
  }
  const ok = await useComponentMessageStore().alert(
    `重置「${props.user.name}」的权限为默认`,
    '将清除该用户的全部个人权限覆盖，恢复到其角色本身的权限。此操作会立即生效，请确认。'
  )
  if (!ok) {
    return
  }
  saving.value = true
  const res = await kunFetch<KunUserPermState>(
    `/admin/user-permissions/${props.user.id}`,
    { method: 'PUT', body: { overrides: [] } }
  )
  saving.value = false
  if (res) {
    applyState(res)
    useMessage('已重置为默认', 'success')
  }
}
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-2xl w-[94vw]">
    <div class="flex max-h-[82dvh] flex-col gap-3">
      <header class="flex flex-wrap items-center gap-2">
        <h2 class="text-xl font-bold">权限调整</h2>
        <KunUserChip :user="user" size="sm" />
      </header>

      <div class="flex flex-wrap items-center gap-2 text-sm">
        <span class="text-default-500">角色:</span>
        <template v-if="roleChips.length">
          <KunChip
            v-for="label in roleChips"
            :key="label"
            size="sm"
            variant="flat"
            color="secondary"
          >
            {{ label }}
          </KunChip>
        </template>
        <span v-else class="text-default-400">无角色</span>
      </div>

      <KunLoading v-if="loading" />

      <template v-else-if="state">
        <KunInfo
          v-if="isRen"
          color="warning"
          title="莲权限不可调整"
          description="莲（ren）恒持全部权限，不可调整。"
        />
        <KunInfo
          v-else-if="rankLocked"
          color="warning"
          title="无法调整该用户"
          description="仅可调整低于自身角色的用户。"
        />
        <KunInfo
          v-else
          color="info"
          description="勾选 = 在角色之外额外授予该用户一项权限；取消勾选 = 撤销一项其角色本有的权限。带有小圆点的行即为相对角色权限的个人覆盖（绿色=个人授予，黄色=个人撤销）。"
        />

        <!-- Grouped permission rows. -->
        <div class="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
          <div v-for="entry in groups" :key="entry.group">
            <div
              class="bg-default-100/60 text-default-600 rounded-md px-2 py-1 text-xs font-semibold"
            >
              {{ entry.group }}
            </div>
            <div
              v-for="perm in entry.perms"
              :key="perm"
              class="border-default-100 flex items-center gap-2 border-b px-2 py-2"
            >
              <div class="relative shrink-0">
                <KunTooltip
                  v-if="possessionLocked(perm)"
                  text="不可增删自己未持有的权限"
                  position="top"
                >
                  <KunCheckBox
                    :model-value="isChecked(perm)"
                    :disabled="true"
                    color="primary"
                  />
                </KunTooltip>
                <KunCheckBox
                  v-else
                  :model-value="isChecked(perm)"
                  :disabled="cellDisabled(perm)"
                  color="primary"
                  @change="(value: boolean) => toggle(perm, value)"
                />
                <KunTooltip
                  v-if="deviation(perm)"
                  :text="
                    deviation(perm) === 'grant'
                      ? '个人授予（高于角色）'
                      : '个人撤销（低于角色）'
                  "
                  position="top"
                >
                  <span
                    :class="
                      cn(
                        'absolute -top-1.5 -right-1.5 block size-2 rounded-full',
                        deviation(perm) === 'grant'
                          ? 'bg-success'
                          : 'bg-warning'
                      )
                    "
                  />
                </KunTooltip>
              </div>
              <span class="text-sm">{{ KUN_PERMISSION_META[perm].label }}</span>
            </div>
          </div>
        </div>

        <!-- Action bar (hidden for the read-only ren / rank-locked cases). -->
        <div
          v-if="!readOnly"
          class="border-default-200 flex flex-wrap items-center justify-between gap-3 border-t pt-3"
        >
          <span class="text-default-500 text-sm">
            <template v-if="isDirty">
              待保存
              <span class="text-warning font-medium">{{ deltas.length }}</span>
              项个人覆盖
            </template>
            <template v-else-if="hasOverrides">
              当前有
              <span class="text-primary font-medium">{{
                state.overrides.length
              }}</span>
              项个人覆盖
            </template>
            <template v-else>该用户暂无个人权限覆盖</template>
          </span>
          <div class="flex flex-wrap items-center gap-2">
            <KunButton
              variant="light"
              color="default"
              :disabled="!isDirty || saving"
              @click="handleDiscard"
            >
              放弃更改
            </KunButton>
            <KunButton
              variant="light"
              color="danger"
              :disabled="!hasOverrides || saving"
              @click="handleReset"
            >
              重置为默认
            </KunButton>
            <KunButton
              color="primary"
              :disabled="!isDirty"
              :loading="saving"
              @click="handleSave"
            >
              <KunIcon name="lucide:check" />
              保存
            </KunButton>
          </div>
        </div>
      </template>

      <KunNull v-else description="无法加载该用户的权限，请稍后重试" />
    </div>
  </KunModal>
</template>
