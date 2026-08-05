<script setup lang="ts">
// Community-primitive comment section (charter step 04) — the unconditional
// galgame comment UI, rendered from Galgame.vue (the legacy comment components
// were retired in charter step 06a). This area's look is the reference the other
// comment areas conform to.
//
// Paging / grouping / optimistic mutation come from useCommunityCommentList. What
// stays here is what only this area has: the tab-panel loading emit, the
// un-ingested-galgame empty-state guard, and the LEGACY deep-link resolve (old
// notification links carry a pre-migration galgame_comment id, which has to be
// mapped through the community map before it can be scrolled to).
const emit = defineEmits<{
  // Surface the (lazy, client-side) fetch state so the tab panel dims while it
  // loads — same contract as the legacy container.
  'update:loading': [boolean]
}>()

const route = useRoute()
const config = useRuntimeConfig()
const gid = parseInt((route.params as { gid: string }).gid)

const target: CommunityCommentTarget = { kind: 'galgame', galgameId: gid }

// See GalgameResource: for a wiki-catalogue game the forum hasn't ingested, hide
// the empty-state (the detail page's 未收录 notice covers it) but KEEP the
// composer — commenting creates the local row, part of the recording funnel.
const galgame = inject<GalgameDetail>('galgame')

const {
  posts,
  status,
  seeded,
  groups,
  isEmpty,
  hasMore,
  loadingMore,
  loadMore,
  handleNewComment,
  handleUpdated,
  handleTombstoned,
  scrollToPost
} = await useCommunityCommentList(target)

watchEffect(() => emit('update:loading', status.value === 'pending'))

const showEmpty = computed(
  () => isEmpty.value && galgame?.is_on_forum !== false
)

// ──────────────────────────────────────────
// Deep-link continuity (charter ruling 9)
// ──────────────────────────────────────────

// Silent legacy-id resolve: a 404 (old content not imported yet) is ignored,
// so a raw $fetch swallows it instead of kunFetch's error toast.
const locateLegacy = async (legacyId: number): Promise<number | null> => {
  try {
    const resp = await $fetch<{ code: number; data?: { post_id: number } }>(
      `${config.public.apiBaseUrl}/api/galgame/${gid}/comments/locate`,
      { query: { legacy_id: legacyId }, credentials: 'include' }
    )
    return resp && resp.code === 0 && resp.data ? resp.data.post_id : null
  } catch {
    return null
  }
}

// Page forward until the target post is loaded (or the list is exhausted), then
// scroll. Capped so a bad id can't run away.
const ensureLoadedAndScroll = async (postId: number) => {
  let guard = 0
  while (
    !posts.value.some((p) => p.id === postId) &&
    hasMore.value &&
    guard < 50
  ) {
    await loadMore()
    guard += 1
  }
  if (posts.value.some((p) => p.id === postId)) {
    scrollToPost(postId)
  }
}

const resolveDeepLink = async () => {
  const commentParam = Number(route.query.comment) || 0
  // The old @-mention notification shape carried a `thread` param alongside a
  // legacy comment id; its presence flags the id as legacy.
  const hasThreadParam = route.query.thread != null
  const hashMatch = (route.hash || '').match(/^#galgame-comment-(\d+)$/)

  if (commentParam) {
    if (hasThreadParam) {
      const postId = await locateLegacy(commentParam)
      if (postId) {
        await ensureLoadedAndScroll(postId)
      }
      return
    }
    // New deep-link: `comment` IS the community post id.
    await ensureLoadedAndScroll(commentParam)
    return
  }

  if (hashMatch) {
    const raw = Number(hashMatch[1])
    // Old external anchors (#galgame-comment-<id>) may carry either a new post
    // id or a legacy id: try it as a post id first, then fall back to locate.
    await ensureLoadedAndScroll(raw)
    if (posts.value.some((p) => p.id === raw)) {
      return
    }
    const mapped = await locateLegacy(raw)
    if (mapped) {
      await ensureLoadedAndScroll(mapped)
    }
  }
}

onMounted(() => {
  // Resolve deep-links client-side, once the first page has seeded. When the
  // comment subtree hydrates lazily (folded tab), the SSR payload has already
  // seeded the page by the time we mount — check the current value instead of
  // an immediate watch: an immediate callback runs synchronously INSIDE the
  // watch() call, so calling the not-yet-assigned stop handle there is a TDZ
  // crash ("Cannot access 'stop' before initialization").
  if (seeded.value) {
    resolveDeepLink()
    return
  }
  const stop = watch(seeded, (ready) => {
    if (ready) {
      stop()
      resolveDeepLink()
    }
  })
})
</script>

<template>
  <div class="space-y-5">
    <KunHeader name="游戏评论" scale="h2">
      <template #endContent>
        <KunLink size="sm" to="/topic/1482">
          Galgame 评论注意事项, 资源失效, 解压密码错误等问题反馈
        </KunLink>
      </template>
    </KunHeader>

    <CommentCommunityComposer :target="target" @submitted="handleNewComment" />

    <KunLoading v-if="status === 'pending'" />

    <KunNull
      v-else-if="showEmpty"
      description="没人评论, 是没人要这个 Galgame 的小只可爱软萌女孩子了吗, 呜呜呜呜呜呜！！"
    />

    <div v-else-if="groups.length" class="space-y-8">
      <CommentCommunityRow
        v-for="group in groups"
        :key="group.root.id"
        :comment="group.root"
        :replies="group.replies"
        :target="target"
        :depth="0"
        @reply-added="handleNewComment"
        @updated="handleUpdated"
        @tombstoned="handleTombstoned"
      />
    </div>

    <KunButton
      v-if="hasMore"
      variant="light"
      color="primary"
      full-width
      :loading="loadingMore"
      @click="loadMore"
    >
      <KunIcon name="lucide:chevron-down" />
      加载更多评论
    </KunButton>

    <!-- Single community-comment flag modal for this section. -->
    <CommentCommunityFlagModal />
  </div>
</template>
