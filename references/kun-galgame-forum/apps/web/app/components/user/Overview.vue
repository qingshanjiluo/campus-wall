<script setup lang="ts">
// The 动态 tab = a profile OVERVIEW (not an infinite feed): the latest few from
// each section, each block linking to its full tab. Reuses the existing
// per-section endpoints + the rich Galgame/评分 cards; text sections render as
// compact rows via UserOverviewSection.
const props = defineProps<{
  userId: number
}>()

const settings = usePersistSettingsStore()
const uid = props.userId
// How many items to preview per section.
const N = 4

const [topics, galgames, ratings, resources, replies, comments] = await Promise.all([
  useKunFetch<{ topics: UserTopic[]; total: number }>(`/user/${uid}/topics`, {
    query: { page: 1, limit: N, type: 'topic', user_id: uid }
  }),
  useKunFetch<{ items: GalgameCard[]; total: number }>(`/user/${uid}/galgames`, {
    query: {
      page: 1,
      limit: N,
      type: 'galgame_publish',
      user_id: uid,
      show_no_resource: settings.showKUNGalgameNoResource
    }
  }),
  useKunFetch<{ rating_data: GalgameRatingCard[]; total: number }>(
    `/user/${uid}/ratings`,
    { query: { page: 1, limit: N, user_id: uid } }
  ),
  useKunFetch<{ resources: UserGalgameResource[]; total: number }>(
    `/user/${uid}/resources`,
    { query: { page: 1, limit: N, type: 'valid', user_id: uid } }
  ),
  useKunFetch<{ replies: UserReply[]; total: number }>(`/user/${uid}/replies`, {
    query: { page: 1, limit: N, type: 'reply_created', user_id: uid }
  }),
  useKunFetch<{ comments: UserComment[]; total: number }>(
    `/user/${uid}/comments`,
    { query: { page: 1, limit: N, type: 'comment_created', user_id: uid } }
  )
])

const topicItems = computed(() =>
  (topics.data.value?.topics ?? []).map((t) => ({
    text: t.title,
    time: t.created,
    href: `/topic/${t.id}`
  }))
)
const resourceItems = computed(() =>
  (resources.data.value?.resources ?? []).map((r) => ({
    text: getPreferredLanguageText(r.galgame_name),
    time: r.created,
    href: `/galgame/${r.galgame_id}`
  }))
)
const replyItems = computed(() =>
  (replies.data.value?.replies ?? []).map((r) => ({
    text: markdownToText(r.content),
    time: r.created,
    href: replyPermalink(`/topic/${r.topic_id}`, r.floor)
  }))
)
const commentItems = computed(() =>
  (comments.data.value?.comments ?? []).map((c) => ({
    text: markdownToText(c.content),
    time: c.created,
    href: commentPermalink(`/topic/${c.topic_id}`, c.id)
  }))
)
const galgameItems = computed(() => galgames.data.value?.items ?? [])
const ratingItems = computed(() => ratings.data.value?.rating_data ?? [])

const isEmpty = computed(
  () =>
    !topicItems.value.length &&
    !galgameItems.value.length &&
    !ratingItems.value.length &&
    !resourceItems.value.length &&
    !replyItems.value.length &&
    !commentItems.value.length
)
</script>

<template>
  <div class="space-y-8">
    <KunNull v-if="isEmpty" description="这只笨蛋萝莉还没有任何动态" />

    <UserOverviewSection
      v-if="topicItems.length"
      title="话题"
      icon="lucide:square-gantt-chart"
      :view-all-href="`/user/${uid}/topic/topic`"
      :items="topicItems"
    />

    <UserOverviewSection
      v-if="galgameItems.length"
      title="Galgame"
      icon="lucide:gamepad-2"
      :view-all-href="`/user/${uid}/galgame/galgame-publish`"
    >
      <GalgameCard :is-transparent="false" :galgames="galgameItems" />
    </UserOverviewSection>

    <UserOverviewSection
      v-if="ratingItems.length"
      title="Gal 评分"
      icon="lucide:star"
      :view-all-href="`/user/${uid}/rating`"
    >
      <GalgameRatingCard :ratings="ratingItems" :is-transparent="false" />
    </UserOverviewSection>

    <UserOverviewSection
      v-if="resourceItems.length"
      title="Gal 资源"
      icon="lucide:package"
      :view-all-href="`/user/${uid}/resource/valid`"
      :items="resourceItems"
    />

    <UserOverviewSection
      v-if="replyItems.length"
      title="回复"
      icon="carbon:reply"
      :view-all-href="`/user/${uid}/reply/reply-created`"
      :items="replyItems"
    />

    <UserOverviewSection
      v-if="commentItems.length"
      title="评论"
      icon="uil:comment-dots"
      :view-all-href="`/user/${uid}/comment/comment-created`"
      :items="commentItems"
    />
  </div>
</template>
