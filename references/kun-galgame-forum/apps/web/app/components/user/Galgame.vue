<script setup lang="ts">
import {
  kunUserGalgameNavItem,
  type KUN_USER_PAGE_GALGAME_TYPE
} from '~/constants/user'

// Single component, two rendering modes:
//
//   - galgame_publish / galgame_contributed / galgame_like
//     → GET /user/:id/galgames returns galgame cards (cover + name +
//       counts). Same shape the rest of the site uses.
//
//   - galgame_comment / galgame_comment_like
//     → GET /user/:id/galgame-comments returns comment rows
//       (content + author + created + parent galgame id). Rendered
//       in a minimal comment-card style — same UX as
//       /user/:id/comment/ for topic comments. Earlier this branch
//       reused the galgame-card endpoint so you saw the parent
//       galgame instead of the actual comment text.
const props = defineProps<{
  userId: number
  type: (typeof KUN_USER_PAGE_GALGAME_TYPE)[number]
}>()

const COMMENT_TYPES = ['galgame_comment', 'galgame_comment_like'] as const

const isCommentMode = computed(() =>
  (COMMENT_TYPES as readonly string[]).includes(props.type)
)

const activeTab = ref(props.type)
const pageData = reactive({
  page: 1,
  limit: 24,
  type: props.type,
  userId: props.userId
})

interface UserGalgameCommentItem {
  id: number
  galgame_id: number
  content: string
  content_html: string
  user: { id: number; name: string; avatar: string }
  created: string
  // The liked tab surfaces placeholder rows for comments that were since
  // deleted/hidden — render a tombstone instead of the (blanked) body.
  deleted: boolean
}

// Two parallel fetches gated by `isCommentMode`. We swap the endpoint
// (and the result shape) instead of branching client-side on a single
// union response — keeps the wire payload tight and the typing clean.
// Global "显示没有下载资源的 Galgame" preference (cookie-persisted, SSR-safe).
// Off (default) hides resource-less galgames; added to the query + watch so a
// toggle re-fetches. (Comment-mode fetch below is unaffected — it lists
// comments, not galgame cards.)
const settings = usePersistSettingsStore()
const { data: galgameData, status: galgameStatus } = await useKunFetch<{
  items: GalgameCard[]
  total: number
}>(() => `/user/${props.userId}/galgames`, {
  query: computed(() => ({
    ...pageData,
    show_no_resource: settings.showKUNGalgameNoResource
  })),
  watch: [
    () => pageData.page,
    () => pageData.type,
    () => settings.showKUNGalgameNoResource
  ],
  immediate: !isCommentMode.value,
  server: !isCommentMode.value
})

// Comment mode uses keyset (cursor) pagination: the first page renders on the
// server, then `加载更多` appends pages via `after=<next_cursor>` until the
// cursor comes back empty (galgame-card mode above keeps numbered pages).
const comments = ref<UserGalgameCommentItem[]>([])
const commentCursor = ref('')
const commentHasMore = ref(true)
const isLoadingMoreComments = ref(false)

const { data: commentData } = await useKunFetch<{
  comments: UserGalgameCommentItem[]
  next_cursor: string
}>(() => `/user/${props.userId}/galgame-comments`, {
  query: computed(() => ({ type: props.type, limit: pageData.limit })),
  watch: [() => props.type],
  immediate: isCommentMode.value,
  server: isCommentMode.value
})

watch(
  commentData,
  (page) => {
    if (!page) {
      return
    }
    comments.value = page.comments
    commentCursor.value = page.next_cursor
    commentHasMore.value = !!page.next_cursor
  },
  { immediate: true }
)

const loadMoreComments = async () => {
  if (
    isLoadingMoreComments.value ||
    !commentHasMore.value ||
    !commentCursor.value
  ) {
    return
  }
  isLoadingMoreComments.value = true
  const next = await kunFetch<{
    comments: UserGalgameCommentItem[]
    next_cursor: string
  }>(`/user/${props.userId}/galgame-comments`, {
    query: {
      type: props.type,
      limit: pageData.limit,
      after: commentCursor.value
    }
  })
  isLoadingMoreComments.value = false
  if (!next) {
    return
  }
  comments.value.push(...next.comments)
  commentCursor.value = next.next_cursor
  commentHasMore.value = !!next.next_cursor
}
</script>

<template>
  <div class="space-y-3">
    <KunTab
      :items="kunUserGalgameNavItem(userId)"
      :model-value="activeTab"
      variant="light"
      size="sm"
      scrollable
    />

    <!-- Galgame-card mode (galgame_publish / galgame_contributed / galgame_like) -->
    <template v-if="!isCommentMode">
      <div
        v-if="galgameData && galgameData.items.length"
        class="flex flex-col space-y-3"
      >
        <GalgameCard :is-transparent="false" :galgames="galgameData.items" />

        <KunPagination
          v-if="galgameData.total > pageData.limit"
          v-model:current-page="pageData.page"
          :total-page="Math.ceil(galgameData.total / pageData.limit)"
          :is-loading="galgameStatus === 'pending'"
        />
      </div>

      <KunNull
        v-if="galgameData && !galgameData.items.length"
        description="这只笨蛋萝莉没有相关的 Galgame"
      />
    </template>

    <!-- Comment-card mode (galgame_comment / galgame_comment_like) -->
    <template v-else>
      <div v-if="comments.length" class="flex flex-col space-y-3">
        <KunCard
          v-for="c in comments"
          :key="c.id"
          :href="c.galgame_id ? `/galgame/${c.galgame_id}?comment=${c.id}` : ''"
          :is-hoverable="!!c.galgame_id"
          content-class="space-y-2"
        >
          <!-- Tombstone: keep the row, gray the body (matches the community
               comment view's [已删除]). -->
          <p v-if="c.deleted" class="text-default-400 text-sm italic">
            [已删除]
          </p>
          <KunContent v-else compact :content="renderKatex(c.content_html)" />

          <div
            class="text-default-500 flex items-center justify-between text-sm"
          >
            <span>
              {{ c.galgame_id ? `评论于 Galgame #${c.galgame_id}` : '评论' }}
            </span>
            <KunTime :time="c.created" type="date" show-year />
          </div>
        </KunCard>

        <div class="flex justify-center pt-1">
          <KunButton
            v-if="commentHasMore"
            variant="light"
            :loading="isLoadingMoreComments"
            @click="loadMoreComments"
          >
            加载更多
          </KunButton>
          <span v-else class="text-default-400 text-sm">没有更多评论了</span>
        </div>
      </div>

      <KunNull
        v-if="!comments.length"
        description="这只笨蛋萝莉在 Galgame 下没有相关的评论"
      />
    </template>
  </div>
</template>
