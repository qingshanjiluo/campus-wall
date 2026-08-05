// Shared state for the ONE global 推话题 modal (mounted at app.vue root). The
// modal must NOT live inside the <TopicFooterUpvote> that sits in the mobile ⋯
// KunPopover: the popover's content is v-if'd and the modal teleports to body,
// so the click on 确定推 counts as a click OUTSIDE the popover — the popover
// closes, the menu row unmounts, and the modal is torn down mid-click before
// its own handler ever runs (the push silently never happens). So the button
// only calls open(); the modal renders at the stable root. Mirrors
// [useReportModal] and the 萌萌点明细 modal (KunTopBarMoemoepointLog).
export interface UpvoteTarget {
  topicId: number
  targetUserId: number
}

const isOpen = ref(false)
const target = ref<UpvoteTarget | null>(null)
// Promise-based so the caller can roll its local count only once a push lands.
// The resolver lives here, in module scope, precisely so it outlives a trigger
// that unmounts while the dialog is open.
let settle: ((pushed: boolean) => void) | null = null

export const useUpvoteModal = () => {
  const open = (t: UpvoteTarget) =>
    new Promise<boolean>((resolve) => {
      target.value = t
      settle = resolve
      isOpen.value = true
    })

  // Resolves exactly once, however the dialog ends (确定推 / 取消 / 背景 / Esc).
  const close = (pushed: boolean) => {
    const resolve = settle
    settle = null
    isOpen.value = false
    resolve?.(pushed)
  }

  return { isOpen, target, open, close }
}
