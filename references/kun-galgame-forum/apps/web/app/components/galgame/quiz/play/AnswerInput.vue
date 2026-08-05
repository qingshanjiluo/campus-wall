<script setup lang="ts">
// Per-type answer collection. The parent (Play) holds a ref and calls
// getSubmitted() / validate() on submit; grading happens server-side.
import type {
  KunRadioOption,
  KunCheckBoxGroupOption
} from '@kungal/ui-vue'

const props = defineProps<{
  type: QuizType
  content: QuizPublicContent
}>()

const singleValue = ref(-1)
const multiValues = ref<number[]>([])
const judgeValue = ref<'true' | 'false' | ''>('')
const fillValues = ref<string[]>([])
const essayText = ref('')

const optionList = computed(
  () => (props.content as QuizPublicSingle).options ?? []
)
const blankCount = computed(
  () => (props.content as QuizPublicFill).blank_count ?? 0
)

// Size the fill inputs to the blank count.
watch(
  blankCount,
  (n) => {
    fillValues.value = Array.from({ length: n }, (_, i) => fillValues.value[i] ?? '')
  },
  { immediate: true }
)

const choiceOptions = computed<KunRadioOption<number>[]>(() =>
  optionList.value.map((o, i) => ({ value: i, label: o }))
)
const checkChoiceOptions = computed<KunCheckBoxGroupOption<number>[]>(
  () => choiceOptions.value
)

const getSubmitted = (): Record<string, unknown> => {
  switch (props.type) {
    case 'single':
      return { value: singleValue.value }
    case 'multiple':
      return { values: [...multiValues.value].sort((a, b) => a - b) }
    case 'judge':
      return { value: judgeValue.value === 'true' }
    case 'fill':
      return { values: fillValues.value.map((v) => v.trim()) }
    case 'essay':
      return { text: essayText.value.trim() }
    default:
      return {}
  }
}

const validate = (): string | null => {
  switch (props.type) {
    case 'single':
      return singleValue.value >= 0 ? null : '请选择一个答案'
    case 'multiple':
      return multiValues.value.length > 0 ? null : '请至少选择一个答案'
    case 'judge':
      return judgeValue.value ? null : '请选择正确或错误'
    case 'fill':
      return fillValues.value.every((v) => v.trim())
        ? null
        : '请填写所有的空'
    case 'essay':
      return essayText.value.trim() ? null : '请写下你的回答'
    default:
      return '未知题型'
  }
}

defineExpose({ getSubmitted, validate })
</script>

<template>
  <div class="space-y-3">
    <KunRadioGroup
      v-if="type === 'single'"
      v-model="singleValue"
      :options="choiceOptions"
      variant="card"
    />

    <KunCheckBoxGroup
      v-else-if="type === 'multiple'"
      v-model="multiValues"
      :options="checkChoiceOptions"
      variant="card"
    />

    <KunRadioGroup
      v-else-if="type === 'judge'"
      v-model="judgeValue"
      :options="[
        { value: 'true', label: '正确' },
        { value: 'false', label: '错误' }
      ]"
      orientation="horizontal"
      variant="card"
    />

    <div v-else-if="type === 'fill'" class="space-y-2">
      <KunInput
        v-for="(_, i) in fillValues"
        :key="i"
        v-model="fillValues[i]"
        :label="`第 ${i + 1} 空`"
        placeholder="填写你的答案"
      />
    </div>

    <KunTextarea
      v-else-if="type === 'essay'"
      v-model="essayText"
      label="你的回答"
      :rows="4"
      placeholder="写下你的回答, 提交后可查看参考答案 (问答题不计分)"
      :maxlength="2000"
      :show-char-count="true"
      auto-grow
    />
  </div>
</template>
