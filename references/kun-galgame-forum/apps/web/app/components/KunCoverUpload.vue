<script setup lang="ts">
// Reusable cover/banner uploader for the content-addressed image service.
// It uploads to `POST /image/cover` (multipart field `file`) and stores the
// returned content-addressed **hash** (`update:modelValue`), keeping the
// resolved CDN url around for preview only.
const props = withDefaults(
  defineProps<{
    modelValue: string
    previewUrl?: string
    label?: string
  }>(),
  {
    previewUrl: '',
    label: '封面'
  }
)

const emits = defineEmits<{
  'update:modelValue': [value: string]
}>()

// Local preview url: seeded from the prop and re-synced when the parent loads
// a different item. This is preview-only — the submitted value is the hash.
const previewUrl = ref(props.previewUrl)
watch(
  () => props.previewUrl,
  (url) => {
    previewUrl.value = url
  }
)

const isUploading = ref(false)

const handleFileChange = async (e: Event) => {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) {
    return
  }

  isUploading.value = true
  const fd = new FormData()
  fd.append('file', file)
  const res = await kunFetch<{ hash: string; url: string }>('/image/cover', {
    method: 'POST',
    body: fd,
    watch: false
  })
  isUploading.value = false
  // Reset the input so re-selecting the same file fires `change` again.
  input.value = ''

  if (res?.hash) {
    emits('update:modelValue', res.hash)
    previewUrl.value = res.url
    useMessage('封面上传成功', 'success')
  } else {
    useMessage('封面上传失败', 'error')
  }
}

const handleRemove = () => {
  emits('update:modelValue', '')
  previewUrl.value = ''
}
</script>

<template>
  <div>
    <label class="mb-1 block text-sm font-medium">{{ label }}</label>
    <div class="flex items-start gap-3">
      <KunImage
        v-if="previewUrl"
        :src="previewUrl"
        class="border-default-200 h-20 w-32 shrink-0 rounded-md border object-cover"
      />
      <div class="flex flex-1 flex-col gap-2">
        <label
          :class="
            cn(
              'border-default-200 bg-default-100 hover:bg-default-200 inline-flex w-fit cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors',
              isUploading && 'pointer-events-none opacity-60'
            )
          "
        >
          <KunIcon name="lucide:image-up" />
          <span>{{ isUploading ? '上传中...' : '选择图片' }}</span>
          <input
            type="file"
            accept="image/*"
            class="hidden"
            :disabled="isUploading"
            @change="handleFileChange"
          />
        </label>
        <KunButton
          v-if="previewUrl || modelValue"
          variant="light"
          color="danger"
          size="sm"
          class-name="w-fit"
          @click="handleRemove"
        >
          移除
        </KunButton>
      </div>
    </div>
  </div>
</template>
