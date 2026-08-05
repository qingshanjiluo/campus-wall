<script setup lang="ts">
import { rateGalgameQuizQualitySchema } from '~/validations/galgame-quiz'

const props = defineProps<{
  quiz: GalgameQuizPlay
  result: QuizAnswerResult
}>()

const emits = defineEmits<{ rated: [result: QuizQualityResult] }>()

// ── reveal, narrowed by quiz.type ─────────────────────────────────
const optionRows = computed(() => {
  if (props.quiz.type === 'single') {
    const a = props.result.answer as QuizFullSingle
    const chosen = (props.result.submitted as { value: number } | null)?.value
    return a.options.map((text, i) => ({
      text,
      correct: i === a.answer,
      chosen: chosen === i
    }))
  }
  if (props.quiz.type === 'multiple') {
    const a = props.result.answer as QuizFullMultiple
    const chosen =
      (props.result.submitted as { values: number[] } | null)?.values ?? []
    return a.options.map((text, i) => ({
      text,
      correct: a.answers.includes(i),
      chosen: chosen.includes(i)
    }))
  }
  return []
})

const judge = computed(() => {
  if (props.quiz.type !== 'judge') return null
  const a = props.result.answer as QuizFullJudge
  const mine = (props.result.submitted as { value: boolean } | null)?.value
  return { answer: a.answer, mine }
})

const fillRows = computed(() => {
  if (props.quiz.type !== 'fill') return []
  const a = props.result.answer as QuizFullFill
  const vals = (props.result.submitted as { values: string[] } | null)?.values ?? []
  return a.blanks.map((b, i) => ({ accepted: b.accepted, mine: vals[i] ?? '' }))
})

const essay = computed(() => {
  if (props.quiz.type !== 'essay') return null
  const a = props.result.answer as QuizFullEssay
  const mine = (props.result.submitted as { text: string } | null)?.text ?? ''
  return { reference: a.reference, mine }
})

const rowClass = (correct: boolean, chosen: boolean) => {
  if (correct) return 'border-success/60 bg-success/10 text-success'
  if (chosen) return 'border-danger/60 bg-danger/10 text-danger'
  return 'border-default-200 text-default-600'
}

// ── quality rating ────────────────────────────────────────────────
const quality = ref(props.result.quality_rating ?? 0)
const isRating = ref(false)

const submitQuality = async () => {
  if (!requireLogin()) return
  const body = { quiz_id: props.quiz.id, quality_rating: quality.value }
  const valid = useKunSchemaValidator(rateGalgameQuizQualitySchema, body)
  if (!valid) return

  isRating.value = true
  const res = await kunFetch<QuizQualityResult>(
    `/galgame-quiz/${props.quiz.id}/quality`,
    { method: 'PUT', body }
  )
  isRating.value = false
  if (res) {
    useMessage('评分成功', 'success')
    emits('rated', res)
  }
}
</script>

<template>
  <div class="space-y-4">
    <!-- correct / incorrect banner (auto-graded types only) -->
    <KunInfo
      v-if="result.is_correct !== null"
      :color="result.is_correct ? 'success' : 'danger'"
      :icon="result.is_correct ? 'lucide:circle-check' : 'lucide:circle-x'"
      :title="result.is_correct ? '回答正确' : '回答错误'"
      :description="
        result.reward_delta > 0 ? `答对啦, +${result.reward_delta} 萌萌点` : ''
      "
    />
    <KunInfo
      v-else-if="quiz.type === 'essay'"
      color="secondary"
      icon="lucide:book-open"
      title="参考答案"
      description="问答题不自动判分, 以下为出题人提供的参考答案"
    />
    <KunInfo
      v-else
      color="secondary"
      icon="lucide:book-open"
      title="本题答案"
      description="你是这道题的出题人, 以下为本题答案与解析"
    />

    <!-- single / multiple reveal -->
    <div v-if="optionRows.length" class="space-y-2">
      <div
        v-for="(row, i) in optionRows"
        :key="i"
        class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm"
        :class="rowClass(row.correct, row.chosen)"
      >
        <span class="break-words whitespace-pre-wrap">{{ row.text }}</span>
        <span class="flex shrink-0 items-center gap-2">
          <KunChip v-if="row.chosen" size="sm" variant="light">你的选择</KunChip>
          <KunIcon v-if="row.correct" name="lucide:check" />
        </span>
      </div>
    </div>

    <!-- judge reveal -->
    <div v-else-if="judge" class="flex flex-wrap items-center gap-3 text-sm">
      <KunChip color="success" variant="flat">
        正确答案: {{ judge.answer ? '正确' : '错误' }}
      </KunChip>
      <KunChip
        :color="judge.mine === judge.answer ? 'success' : 'danger'"
        variant="light"
      >
        你的回答: {{ judge.mine ? '正确' : '错误' }}
      </KunChip>
    </div>

    <!-- fill reveal -->
    <div v-else-if="fillRows.length" class="space-y-2">
      <div
        v-for="(row, i) in fillRows"
        :key="i"
        class="border-default-200 space-y-1 rounded-lg border px-3 py-2 text-sm"
      >
        <p class="text-default-500">第 {{ i + 1 }} 空</p>
        <p class="text-success">可接受答案: {{ row.accepted.join(' / ') }}</p>
        <p class="text-default-700">你的回答: {{ row.mine || '(未填写)' }}</p>
      </div>
    </div>

    <!-- essay reveal -->
    <div v-else-if="essay" class="space-y-2 text-sm">
      <div class="border-default-200 rounded-lg border px-3 py-2">
        <p class="text-default-500 mb-1">参考答案</p>
        <p class="text-default-700 break-words whitespace-pre-wrap">
          {{ essay.reference }}
        </p>
      </div>
      <div
        v-if="essay.mine"
        class="border-default-200 rounded-lg border px-3 py-2"
      >
        <p class="text-default-500 mb-1">你的回答</p>
        <p class="text-default-700 break-words whitespace-pre-wrap">
          {{ essay.mine }}
        </p>
      </div>
    </div>

    <!-- explanation -->
    <div
      v-if="result.explanation"
      class="bg-default-100 rounded-lg px-3 py-2 text-sm"
    >
      <p class="text-default-500 mb-1 flex items-center gap-1">
        <KunIcon name="lucide:lightbulb" />解析
      </p>
      <p class="text-default-700 break-words whitespace-pre-wrap">
        {{ result.explanation }}
      </p>
    </div>

    <KunDivider />

    <!-- quality rating (answerers only; the author cannot rate own quiz) -->
    <div v-if="!quiz.is_author" class="space-y-2">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <label class="text-sm font-medium">给这道题的质量打分 (1-10)</label>
        <span v-if="quiz.quality_count > 0" class="text-default-500 text-sm">
          平均 {{ quiz.quality_average }} 分 · {{ quiz.quality_count }} 人评分
        </span>
      </div>
      <div class="flex flex-wrap items-center gap-4">
        <KunRating v-model="quality" :max="10" aria-label="quiz-quality" />
        <span class="text-warning-500 text-2xl font-bold">{{ quality }}</span>
        <KunButton
          :loading="isRating"
          :disabled="quality < 1"
          size="sm"
          @click="submitQuality"
        >
          {{ result.quality_rating ? '更新评分' : '提交评分' }}
        </KunButton>
      </div>
    </div>
  </div>
</template>
