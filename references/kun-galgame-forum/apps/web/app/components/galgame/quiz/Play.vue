<script setup lang="ts">
import {
  KUN_QUIZ_TYPE_MAP,
  KUN_QUIZ_TYPE_ICON_MAP,
  KUN_QUIZ_TYPE_COLOR_MAP,
  KUN_QUIZ_CATEGORY_MAP,
  KUN_QUIZ_SPOILER_MAP,
  KUN_QUIZ_SPOILER_COLOR_MAP,
  kunQuizDifficultyLabel,
  kunQuizDifficultyColor
} from '~/constants/galgame-quiz'
import { answerGalgameQuizSchema } from '~/validations/galgame-quiz'

const props = defineProps<{ quiz: GalgameQuizPlay }>()

const router = useRouter()
// 返回题库: prefer a history round-trip so the exact browse page + filters +
// scroll position the user left are restored (the list is fully URL-driven —
// see quiz/Container.vue). Fall back to the library root for deep links or
// arrivals from the galgame-detail quiz panel, where the previous history entry
// isn't the list.
const returnToLibrary = () => {
  const back = window.history.state?.back
  if (typeof back === 'string' && /^\/galgame-quiz(\?|#|$)/.test(back)) {
    router.back()
  } else {
    navigateTo('/galgame-quiz')
  }
}

// Local mutable copy so answering / rating updates the view without a refetch.
const state = ref<GalgameQuizPlay>({ ...props.quiz })
// Edit fetches the answer key (quiz.edit_any); delete removes the quiz and its
// records (quiz.delete_any). The author holds both over their own quiz.
const canEditAnyQuiz = useCan('quiz.edit_any')
const canDeleteAnyQuiz = useCan('quiz.delete_any')

const answerRef = ref<{
  getSubmitted: () => Record<string, unknown>
  validate: () => string | null
} | null>(null)
const isSubmitting = ref(false)

const submitAnswer = async () => {
  if (!requireLogin()) return
  const err = answerRef.value?.validate()
  if (err) {
    useMessage(err, 'warn')
    return
  }
  const submitted = answerRef.value?.getSubmitted() ?? {}
  const body = { quiz_id: state.value.id, submitted }
  const valid = useKunSchemaValidator(answerGalgameQuizSchema, body)
  if (!valid) return

  isSubmitting.value = true
  const res = await kunFetch<QuizAnswerResult>(
    `/galgame-quiz/${state.value.id}/answer`,
    { method: 'POST', body }
  )
  isSubmitting.value = false
  if (res) {
    state.value.my_answer = res
    state.value.answer_count += 1
    if (res.is_correct) state.value.correct_count += 1
    if (res.reward_delta > 0) {
      useMessage(`回答正确, +${res.reward_delta} 萌萌点`, 'success')
    }
    // Hidden linked games are revealed once answered — refetch to populate them.
    if (state.value.hide_galgame && !state.value.galgames.length) {
      const fresh = await kunFetch<GalgameQuizPlay>(
        `/galgame-quiz/${state.value.id}`
      )
      if (fresh) state.value.galgames = fresh.galgames
    }
  }
}

const onRated = (r: QuizQualityResult) => {
  state.value.quality_average = r.quality_average
  state.value.quality_count = r.quality_count
  if (state.value.my_answer) {
    state.value.my_answer.quality_rating = r.quality_rating
  }
}

const isDeleting = ref(false)
const canEdit = computed(() => state.value.is_author || canEditAnyQuiz.value)
const canDelete = computed(
  () => state.value.is_author || canDeleteAnyQuiz.value
)
const canManage = computed(() => canEdit.value || canDelete.value)

const showEdit = ref(false)
const editData = ref<QuizEditData | null>(null)
const openEdit = async () => {
  const data = await kunFetch<QuizEditData>(
    `/galgame-quiz/${state.value.id}/edit`
  )
  if (data) {
    editData.value = data
    showEdit.value = true
  }
}
// After an edit, refetch the play detail so the view reflects the changes.
const reloadQuiz = async () => {
  const fresh = await kunFetch<GalgameQuizPlay>(
    `/galgame-quiz/${state.value.id}`
  )
  if (fresh) state.value = fresh
}

const remove = async () => {
  const ok = await useComponentMessageStore().alert(
    '确认删除',
    '删除后本题及所有作答记录将被移除, 出题获得的萌萌点会被扣除'
  )
  if (!ok) return
  isDeleting.value = true
  const res = await kunFetch(
    `/galgame-quiz/${state.value.id}?quiz_id=${state.value.id}`,
    { method: 'DELETE' }
  )
  isDeleting.value = false
  if (res) {
    useMessage('已删除', 'success')
    returnToLibrary()
  }
}

const correctRate = computed(() =>
  state.value.answer_count > 0
    ? Math.round((state.value.correct_count / state.value.answer_count) * 100)
    : null
)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-2">
      <KunButton variant="light" size="sm" @click="returnToLibrary">
        <span class="flex items-center gap-1">
          <KunIcon name="lucide:arrow-left" />返回题库
        </span>
      </KunButton>

      <KunPopover v-if="canManage" position="bottom-end">
        <template #trigger>
          <KunButton :is-icon-only="true" variant="light" size="sm">
            <KunIcon name="lucide:ellipsis" />
          </KunButton>
        </template>
        <div class="flex w-32 flex-col gap-1 p-2">
          <KunButton
            v-if="canEdit"
            variant="light"
            color="default"
            size="sm"
            class-name="w-full justify-start gap-2"
            @click="openEdit"
          >
            <KunIcon name="lucide:pencil" />编辑
          </KunButton>
          <KunButton
            v-if="canDelete"
            variant="light"
            color="danger"
            size="sm"
            class-name="w-full justify-start gap-2"
            :loading="isDeleting"
            @click="remove"
          >
            <KunIcon name="lucide:trash-2" />删除
          </KunButton>
        </div>
      </KunPopover>
    </div>

    <KunCard :is-transparent="false">
      <div class="space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <KunChip :color="KUN_QUIZ_TYPE_COLOR_MAP[state.type]" variant="flat">
            <span class="flex items-center gap-1">
              <KunIcon :name="KUN_QUIZ_TYPE_ICON_MAP[state.type]" />
              {{ KUN_QUIZ_TYPE_MAP[state.type] }}
            </span>
          </KunChip>
          <KunChip
            :color="kunQuizDifficultyColor(state.difficulty)"
            variant="flat"
          >
            {{ kunQuizDifficultyLabel(state.difficulty) }} ·
            {{ state.difficulty }}
          </KunChip>
          <KunChip variant="light">
            {{ KUN_QUIZ_CATEGORY_MAP[state.category] }}
          </KunChip>
          <KunChip
            v-if="state.spoiler_level !== 'none'"
            :color="KUN_QUIZ_SPOILER_COLOR_MAP[state.spoiler_level]"
            variant="flat"
          >
            {{ KUN_QUIZ_SPOILER_MAP[state.spoiler_level] }}
          </KunChip>
          <KunLink
            v-for="g in state.galgames"
            :key="g.id"
            :to="`/galgame/${g.id}`"
            class="text-sm"
          >
            {{ getPreferredLanguageText(g.name) }}
          </KunLink>
          <KunChip
            v-if="
              !state.galgames.length && state.hide_galgame && !state.my_answer
            "
            variant="flat"
            size="sm"
          >
            <span class="flex items-center gap-1">
              <KunIcon name="lucide:lock" />关联作品作答后揭晓
            </span>
          </KunChip>
        </div>

        <h1 class="text-xl font-bold break-words whitespace-pre-wrap">
          {{ state.question }}
        </h1>

        <!-- author's markdown description / image hints -->
        <KunContent
          v-if="state.description_html"
          :content="renderKatex(state.description_html)"
        />

        <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div
            class="text-default-500 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm"
          >
            <span class="text-default-700 flex items-center gap-1">
              <KunAvatar
                :disable-floating="true"
                :user="state.user"
                size="xs"
                :is-navigation="false"
              />
              {{ state.user.name }}
            </span>
            <KunTime :time="state.created" />
            <span class="flex items-center gap-1">
              <KunIcon name="lucide:users" />{{ state.answer_count }} 人作答
            </span>
            <span v-if="correctRate !== null" class="flex items-center gap-1">
              <KunIcon name="lucide:target" />正确率 {{ correctRate }}%
            </span>
          </div>

          <!-- view + favorite, KunReaction-styled, right-aligned (stays right
               on its own line when the row wraps on mobile). -->
          <div class="text-default-500 ml-auto flex items-center gap-1">
            <span class="inline-flex items-center gap-1.5 px-2 py-1 text-sm">
              <KunIcon name="lucide:eye" class="text-[1.15rem]" />{{
                state.view
              }}
            </span>
            <FavoriteToggle
              :favorited="state.is_favorited"
              :count="state.favorite_count"
              :endpoint="`/galgame-quiz/${state.id}/favorite`"
              :messages="['已收藏', '已取消收藏']"
            />
          </div>
        </div>

        <KunDivider />

        <!-- answer flow -->
        <div v-if="!state.my_answer" class="space-y-4">
          <GalgameQuizPlayAnswerInput
            ref="answerRef"
            :type="state.type"
            :content="state.content"
          />
          <div class="flex justify-end">
            <KunButton :loading="isSubmitting" @click="submitAnswer">
              {{ state.type === 'essay' ? '提交并查看参考答案' : '提交答案' }}
            </KunButton>
          </div>
        </div>

        <!-- result / reveal -->
        <GalgameQuizPlayResult
          v-else
          :quiz="state"
          :result="state.my_answer"
          @rated="onRated"
        />

        <KunDivider />

        <!-- 查看题目详情: stats + answerer records (peek → expand) -->
        <GalgameQuizDetailPanel :quiz="state" />
      </div>
    </KunCard>

    <GalgameQuizPublish
      v-model="showEdit"
      :edit-data="editData"
      @on-updated="reloadQuiz"
    />
  </div>
</template>
