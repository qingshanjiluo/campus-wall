<script setup lang="ts">
// Thin modal shell around GalgameQuizForm — the in-context create (the 出题
// button on /galgame-quiz) and the edit flow (detail page ⋯ → 编辑). The
// dedicated /edit/galgame/quiz page renders the same form without this shell.
const props = defineProps<{
  modelValue: boolean
  galgameId?: number
  editData?: QuizEditData | null
}>()

const emits = defineEmits<{
  'update:modelValue': [value: boolean]
  onPublished: [quiz: GalgameQuizCard]
  onUpdated: []
}>()

const close = () => emits('update:modelValue', false)
</script>

<template>
  <KunModal
    :model-value="modelValue"
    inner-class-name="max-w-[720px] w-[90vw]"
    :is-dismissable="false"
    @update:model-value="(v) => emits('update:modelValue', v)"
  >
    <GalgameQuizForm
      :galgame-id="props.galgameId"
      :edit-data="props.editData"
      @published="
        (q) => {
          emits('onPublished', q)
          close()
        }
      "
      @updated="
        () => {
          emits('onUpdated')
          close()
        }
      "
      @cancel="close"
    />
  </KunModal>
</template>
