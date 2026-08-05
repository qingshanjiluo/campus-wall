<script setup lang="ts">
// The 题库 tab on a galgame detail page — quizzes bound to this game, plus an
// 出题 entry that pre-binds the new quiz to the game (galgameId).
const route = useRoute()
const gid = computed(() => parseInt((route.params as { gid: string }).gid))

const emit = defineEmits<{
  // Surface fetch state so the tab panel dims while (re)loading.
  'update:loading': [boolean]
}>()

const params = reactive({ page: 1, limit: 12, galgame_id: gid.value })
const { data, status, refresh } = await useKunFetch<QuizListPage>(
  '/galgame-quiz/all',
  { method: 'GET', query: params }
)
watchEffect(() => emit('update:loading', status.value === 'pending'))

const showPublish = ref(false)
const openPublish = () => {
  if (!requireLogin()) return
  showPublish.value = true
}
const onPublished = () => {
  params.page = 1
  refresh()
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between gap-2">
      <KunHeader name="本作题库" scale="h3">
        <template #description>
          <p class="text-default-500 text-sm">
            与本作相关的题目, 也欢迎为本作出题
          </p>
        </template>
      </KunHeader>
      <KunButton size="sm" @click="openPublish">
        <span class="flex items-center gap-1">
          <KunIcon name="lucide:plus" />出题
        </span>
      </KunButton>
    </div>

    <GalgameQuizList
      v-if="data && data.quiz_data.length"
      :quizzes="data.quiz_data"
    />
    <KunNull
      v-else-if="status !== 'pending'"
      description="本作还没有题目, 快来出第一题吧"
    />

    <KunPagination
      v-if="(data?.total || 0) > params.limit"
      v-model:current-page="params.page"
      :total-page="Math.ceil((data?.total || 0) / params.limit)"
      :is-loading="status === 'pending'"
    />

    <GalgameQuizPublish
      v-model="showPublish"
      :galgame-id="gid"
      @on-published="onPublished"
    />
  </div>
</template>
