// Shared open-state for the ONE community-comment flag modal, mounted once inside
// GalgameCommentCommunityContainer. A per-comment 举报 entry only calls open(id);
// the modal itself renders at the container root. Mirrors useReportModal — the
// modal must not live next to the action bar that triggers it, or a re-render of
// that row would tear it down.
const isOpen = ref(false)
const targetPostId = ref<number | null>(null)

export const useGalgameCommentFlag = () => {
  const open = (postId: number) => {
    targetPostId.value = postId
    isOpen.value = true
  }
  return { isOpen, targetPostId, open }
}
