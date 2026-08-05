<script setup lang="ts">
// Shared drag-drop + multi-file picker for the cover/screenshot editors.
// Dumb: it only surfaces the picked/dropped File[]; the parent uploads +
// de-dups (useGalgameImageUpload). Doubles as the always-visible "add" tile at
// the end of the grid, so adding more images never reopens a separate dialog.
withDefaults(
  defineProps<{
    label: string
    isUploading?: boolean
    progressText?: string
    accept?: string
  }>(),
  {
    isUploading: false,
    progressText: '上传中',
    accept: 'image/jpeg,image/png,image/webp'
  }
)

const emit = defineEmits<{ files: [File[]] }>()

const fileInput = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)

const pick = () => fileInput.value?.click()

const onChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = '' // reset so re-picking the same selection still fires
  if (files.length) emit('files', files)
}

const onDrop = (e: DragEvent) => {
  isDragging.value = false
  const files = Array.from(e.dataTransfer?.files ?? [])
  if (files.length) emit('files', files)
}
</script>

<template>
  <div
    role="button"
    tabindex="0"
    :aria-label="label"
    :aria-busy="isUploading"
    class="border-default-300 text-default-500 hover:border-primary hover:text-primary flex aspect-video min-h-28 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-lg border-2 border-dashed p-3 text-center text-sm transition-colors"
    :class="{ 'border-primary text-primary bg-primary/5': isDragging }"
    @click="pick"
    @keydown.enter.prevent="pick"
    @keydown.space.prevent="pick"
    @dragover.prevent="isDragging = true"
    @dragenter.prevent="isDragging = true"
    @dragleave.prevent="isDragging = false"
    @drop.prevent="onDrop"
  >
    <KunIcon
      :name="isUploading ? 'lucide:loader-circle' : 'lucide:image-plus'"
      :class="cn('text-2xl', isUploading && 'animate-spin')"
    />
    <span>{{ isUploading ? progressText : label }}</span>
    <span v-if="!isUploading" class="text-default-400 text-xs">
      点击或拖拽到此 · 可多选
    </span>

    <input
      ref="fileInput"
      type="file"
      :accept="accept"
      multiple
      class="hidden"
      @change="onChange"
    />
  </div>
</template>
