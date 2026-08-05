// Global login / register modal.
//
// A SINGLE <KunAuthModal> is mounted in app.vue and bound to this shared state,
// so any login-gated action anywhere can open the exact same modal the top-bar
// 登录 button uses — instead of each call site inventing its own toast. The
// modal itself just hands off to the OAuth account-center (see AuthModal.vue).
export const useAuthModal = () => {
  // useState keys the toggle globally + SSR-safe (defaults closed on render).
  const isOpen = useState('kun-auth-modal-open', () => false)

  return {
    isOpen,
    open: () => {
      isOpen.value = true
    },
    close: () => {
      isOpen.value = false
    }
  }
}

// Gate a login-required action. Returns true when the user is logged in;
// otherwise pops the auth modal and returns false. Use at the top of any
// click handler that needs auth:
//
//   const handleLike = async () => {
//     if (!requireLogin()) return
//     ...
//   }
//
// This replaces the old `if (!id) { useMessage('请登录…'); return }` pattern so
// a logged-out click opens the login modal rather than a dismissible toast.
export const requireLogin = (): boolean => {
  if (usePersistUserStore().id) {
    return true
  }
  useAuthModal().open()
  return false
}
