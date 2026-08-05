<script setup lang="ts">
import { KUN_UPDATE_LOG, KUN_UPDATE_LOG_TYPE_MAP } from '~/constants/update'
import {
  createUpdateLogSchema,
  updateUpdateLogSchema
} from '~/validations/update-log'
import type { UpdateUpdateLogPayload } from './types'

const props = defineProps<{
  modelValue: boolean
  initialData?: UpdateUpdateLogPayload
}>()

const emits = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [data: UpdateUpdateLogPayload]
}>()

const isModalOpen = computed({
  get: () => props.modelValue,
  set: (value) => emits('update:modelValue', value)
})

const isEditing = computed(() => !!props.initialData?.update_log_id)
const isSubmitting = ref(false)

const logTypeOptions = KUN_UPDATE_LOG.map((type) => ({
  label: KUN_UPDATE_LOG_TYPE_MAP[type] ?? '',
  value: type ?? 'feat'
}))

const getInitialFormData = (): UpdateUpdateLogPayload => ({
  update_log_id: 0,
  version: '',
  type: 'feat',
  content_en_us: '',
  content_ja_jp: '',
  content_zh_cn: '',
  content_zh_tw: '',
  ...(props.initialData || {})
})

const formData = reactive<UpdateUpdateLogPayload>(getInitialFormData())

watch(
  () => isModalOpen.value,
  (isOpen) => {
    if (isOpen) {
      isSubmitting.value = false
      Object.assign(formData, getInitialFormData())
    }
  }
)

const handleSubmit = () => {
  isSubmitting.value = true
  const schema = isEditing.value ? updateUpdateLogSchema : createUpdateLogSchema

  const result = schema.safeParse(formData)

  if (!result.success) {
    const message = JSON.parse(result.error.message)[0]
    useMessage(
      formatKunZodIssue(message),
      'warn'
    )
    isSubmitting.value = false
    return
  }

  emits('submit', { update_log_id: formData.update_log_id, ...result.data })
  isSubmitting.value = false
  isModalOpen.value = false
}
</script>

<template>
  <KunModal
    :is-dismissable="false"
    v-model="isModalOpen"
    inner-class-name="max-w-xl"
  >
    <form @submit.prevent>
      <h2 class="mb-6 text-xl font-bold">
        {{ isEditing ? '编辑更新日志' : '创建新更新日志' }}
      </h2>

      <div class="space-y-4">
        <KunInput v-model="formData.version" label="版本号" required />
        <KunSelect
          v-model="formData.type"
          :options="logTypeOptions"
          label="日志类型"
          required
        />
        <KunTextarea
          v-model="formData.content_zh_cn"
          label="简体中文内容 (1000 字符之内)"
          :rows="5"
        />
        <KunTextarea
          v-model="formData.content_zh_tw"
          label="繁体中文内容 (1000 字符之内)"
          :rows="5"
        />
        <KunTextarea
          v-model="formData.content_en_us"
          label="英文内容 (1000 字符之内)"
          :rows="5"
        />
        <KunTextarea
          v-model="formData.content_ja_jp"
          label="日文内容 (1000 字符之内)"
          :rows="5"
        />
      </div>

      <div class="mt-6 flex justify-end gap-3">
        <KunButton variant="light" color="danger" @click="isModalOpen = false">
          取消
        </KunButton>
        <KunButton
          @click="handleSubmit"
          color="primary"
          :loading="isSubmitting"
        >
          {{ isEditing ? '保存更改' : '创建' }}
        </KunButton>
      </div>
    </form>
  </KunModal>
</template>
