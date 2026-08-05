<script setup lang="ts">
import { createToolsetResourceSchema } from '~/validations/toolset'

const props = defineProps<{
  toolsetId: number
  type: 's3' | 'user'
  uploadResult: ToolsetUploadResult
}>()

const emits = defineEmits<{
  onClose: []
  onSuccess: [ToolsetResource]
}>()

// In s3 mode the file pointer is the S3 key returned by the upload
// pipeline (resource.content gets stored as-is by the API and is treated
// as immutable for s3-type rows). In user mode the user types a link
// into the content textarea below.
//
// formData.size is stored as a raw byte-count string in s3 mode (e.g.
// "1572864") and as a human-readable "1007MB" / "520KB" in user mode.
// Why: Item.vue (the resource list) renders s3 rows by doing
// `formatFileSize(Number(size))`, so persisting a pre-formatted string
// would round-trip back through Number(...) as NaN. The display value
// in this form is computed separately so users still see "1.5 MB"
// rather than a raw byte integer.
const formData = reactive({
  toolset_id: props.toolsetId,
  type: props.type,
  // s3: content stays empty — the download URL is resolved server-side from the
  // artifact uuid. user: the link typed into the textarea below.
  content: '',
  artifact_uuid: props.type === 's3' ? props.uploadResult.artifact_uuid : '',
  size:
    props.type === 's3' && props.uploadResult.size
      ? String(props.uploadResult.size)
      : '',
  code: '',
  password: '',
  note: ''
})
const isLoading = ref(false)

const sizeDisplay = computed(() => {
  if (props.type === 's3') {
    const bytes = Number(formData.size)
    return Number.isFinite(bytes) && bytes > 0 ? formatFileSize(bytes) : ''
  }
  return formData.size
})

const onSizeInput = (value: string | number) => {
  // s3 mode field is disabled — only user mode writes back to formData.
  if (props.type === 'user') {
    formData.size = String(value)
  }
}

watch(
  () => props.type,
  () => {
    formData.type = props.type
    // Switching modes resets content/uuid + size — s3 rebinds to upload data,
    // user mode clears so the inputs start empty for manual entry.
    if (props.type === 's3') {
      formData.artifact_uuid = props.uploadResult.artifact_uuid
      formData.content = ''
      formData.size = props.uploadResult.size
        ? String(props.uploadResult.size)
        : ''
    } else {
      formData.artifact_uuid = ''
      formData.content = ''
      formData.size = ''
    }
  }
)

watch(
  () => props.uploadResult,
  () => {
    if (props.type === 's3') {
      formData.artifact_uuid = props.uploadResult.artifact_uuid
      formData.size = props.uploadResult.size
        ? String(props.uploadResult.size)
        : ''
    }
  }
)

const submitLink = async () => {
  const result = useKunSchemaValidator(createToolsetResourceSchema, formData)
  if (!result) {
    return
  }

  isLoading.value = true
  const ok = await kunFetch<ToolsetResource>(
    `/toolset/${props.toolsetId}/resource`,
    {
      method: 'POST',
      body: formData
    }
  )
  isLoading.value = false

  if (ok) {
    useMessage('资源发布成功', 'success')
    emits('onSuccess', ok)
    emits('onClose')
  }
}
</script>

<template>
  <div class="space-y-3">
    <KunInput
      :placeholder="
        props.type === 'user'
          ? '大小 (如 520KB, 1007MB, 0721GB)'
          : '确认上传完成后, 自动生成文件大小'
      "
      :disabled="props.type === 's3'"
      :model-value="sizeDisplay"
      @update:model-value="onSizeInput"
    />
    <KunInput
      v-if="props.type === 'user'"
      placeholder="提取码 (可选)"
      v-model="formData.code"
    />
    <KunInput placeholder="解压密码 (可选)" v-model="formData.password" />
    <KunTextarea
      placeholder="备注 (建议写明您提供的资源的使用注意事项等)"
      v-model="formData.note"
    />
    <KunTextarea
      v-if="props.type === 'user'"
      placeholder="资源链接 (如果您的自定义链接有多个, 请使用英文逗号分隔每个链接)"
      v-model="formData.content"
    />
    <div class="flex justify-end gap-2">
      <KunButton variant="light" color="danger" @click="emits('onClose')">
        取消
      </KunButton>
      <KunButton :loading="isLoading" :disabled="isLoading" @click="submitLink">
        提交链接
      </KunButton>
    </div>
  </div>
</template>
