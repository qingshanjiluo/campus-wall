<script setup lang="ts">
// Logout scope chooser. Mounted at the app.vue root (a stable, NON-scoped
// node) — NOT inside the avatar popover, whose content is v-if'd and unmounts
// the instant the user clicks into the modal (which would tear the modal down
// with it — the cause of "点退出登录没有反应"). UserInfo's 退出登录 sets the
// temp-store flag; this self-binds to it, so the modal survives the popover.
//
// It lives at app.vue and not the avatar bar specifically because this
// component's root IS <KunModal> (a <Teleport> root). A <style scoped> parent
// would try to stamp its scope id onto our teleport root for deep styling and
// Vue would warn ("Extraneous non-props attributes … at <KunModal>"); app.vue's
// style is global, so — like KunAuthModal / KunCapture there — no warning.
const { showKUNGalgameLogout: isOpen } = storeToRefs(useTempSettingStore())

const logoutPending = ref<'local' | 'everywhere' | null>(null)

// "This site only" — end the forum's own BFF session server-side via
// POST /api/auth/logout (revoke the OAuth refresh token + delete the Redis
// session + clear the kungal_session cookie), THEN reset the client store. The
// central OAuth (SSO) session stays, so re-login here is silent. Without the
// backend call the cookie + Redis session survived and a hard refresh re-logged
// the user straight back in (resetUser only clears the client-side store).
const logoutLocal = async () => {
  if (logoutPending.value) return
  logoutPending.value = 'local'
  await kunFetch('/auth/logout', { method: 'POST' })
  usePersistUserStore().resetUser()
  useMessage(10110, 'success')
  isOpen.value = false
  logoutPending.value = null
}

// "Everywhere" — also end the central OP session via RP-initiated logout so no
// site can silently re-login. End the forum's own session first (same backend
// call), then top-level navigate to the OP logout entrypoint. See
// docs/oauth/07-logout.md.
const logoutEverywhere = async () => {
  if (logoutPending.value) return
  logoutPending.value = 'everywhere'
  await kunFetch('/auth/logout', { method: 'POST' })
  usePersistUserStore().resetUser()
  // Full logout ends the OP session bag → the local account roster is now stale;
  // forget it so a shared device leaves nothing behind. (仅退出本站 keeps it.)
  useKnownAccounts().clearAll()
  startOAuthLogout() // top-level redirect to the OP; clears the SSO session
}
</script>

<template>
  <KunModal v-model="isOpen" inner-class-name="max-w-lg">
    <div class="space-y-4 p-2">
      <div class="space-y-1">
        <h2 class="text-foreground text-lg font-semibold">退出登录</h2>
        <p class="text-default-500 text-sm">请选择退出范围：</p>
      </div>

      <button
        type="button"
        :disabled="!!logoutPending"
        class="border-primary-200 bg-primary-50/50 hover:bg-primary-100/60 w-full rounded-xl border p-4 text-left transition-colors disabled:opacity-60"
        @click="logoutEverywhere"
      >
        <div class="flex items-start gap-3">
          <KunIcon
            :name="logoutPending === 'everywhere' ? 'lucide:loader-circle' : 'lucide:log-out'"
            :class="`text-primary mt-0.5 size-5 shrink-0 ${logoutPending === 'everywhere' ? 'animate-spin' : ''}`"
          />
          <div class="space-y-1">
            <div class="text-foreground flex items-center gap-2 font-medium">
              退出本站和 OAuth 账号
              <span class="bg-primary-100 text-primary-700 rounded px-1.5 py-0.5 text-xs">推荐</span>
            </div>
            <p class="text-default-500 text-xs leading-relaxed">
              本站与 OAuth 账号都会退出；其它已登录的站点会在下次刷新登录态时一并退出；再次登录需重新验证身份。适合公共 / 共享设备。
            </p>
          </div>
        </div>
      </button>

      <button
        type="button"
        :disabled="!!logoutPending"
        class="border-default-200 hover:bg-default-100 w-full rounded-xl border p-4 text-left transition-colors disabled:opacity-60"
        @click="logoutLocal"
      >
        <div class="flex items-start gap-3">
          <KunIcon
            :name="logoutPending === 'local' ? 'lucide:loader-circle' : 'lucide:monitor'"
            :class="`text-default-500 mt-0.5 size-5 shrink-0 ${logoutPending === 'local' ? 'animate-spin' : ''}`"
          />
          <div class="space-y-1">
            <div class="text-foreground font-medium">仅退出本站</div>
            <p class="text-default-500 text-xs leading-relaxed">
              只退出本站；OAuth 账号与其它站点保持登录；再次登录本站可免密直接进入。适合自己的设备。
            </p>
          </div>
        </div>
      </button>

      <div class="flex justify-end pt-1">
        <KunButton
          variant="light"
          :disabled="!!logoutPending"
          @click="isOpen = false"
        >
          取消
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
