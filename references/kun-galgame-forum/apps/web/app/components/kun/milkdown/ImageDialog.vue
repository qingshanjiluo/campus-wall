<script setup lang="ts">
// Host-built image dialog shared by EVERY image-capable forum editor — the
// topic-edit page AND the reply / comment / toolset editors (the latter via the
// <KunMilkdownDualEditorProvider> shim). <KunEditor> ships only the primitives
// (insert an image from a ready URL, or upload a File) — the dialog UI is the
// host's job (the standard headless split, same as TipTap). Supports: paste a
// URL, click/drag to upload one or many images, and re-insert from a recent-
// images history. Rendered in the #toolbar slot in place of the built-in image
// button (which is why KUN_EDITOR_TOOLBAR_ITEMS omits 'image'). Clipboard/drag
// image PASTE into the doc is handled by <KunEditor> itself via the uploadImage
// adapter — this dialog adds URL insert, multi-upload, and history on top.
import type { KunEditorToolbarApi } from '@kungal/editor-vue'

const props = defineProps<KunEditorToolbarApi>()

// Upload via the forum's own adapter (→ /image/topic → a /image/<hash> token) so
// we get the URL back to both insert AND remember; api.uploadImage would insert
// but not hand us the token for the history.
const { uploadImage } = useKunEditorAdapters()
const history = usePersistImageHistoryStore()

const url = ref('')
const isUploading = ref(false)
const progressText = ref('上传中')
const popover = ref<{ close: () => void } | null>(null)

const insertUrl = () => {
  const src = url.value.trim()
  if (!src) {
    return
  }
  props.insertImage({ src })
  history.add(src)
  url.value = ''
  popover.value?.close()
}

const uploadFiles = async (files: File[]) => {
  if (isUploading.value || !uploadImage) {
    return
  }
  const images = files.filter((file) => file.type.startsWith('image/'))
  if (!images.length) {
    return
  }
  isUploading.value = true
  let done = 0
  try {
    for (const file of images) {
      const src = await uploadImage(file)
      props.insertImage({ src })
      history.add(src)
      progressText.value = `上传中 ${++done}/${images.length}`
    }
    popover.value?.close()
  } catch {
    useMessage('图片上传失败, 请重试', 'error')
  } finally {
    isUploading.value = false
    progressText.value = '上传中'
  }
}

const insertFromHistory = (src: string) => {
  props.insertImage({ src })
  history.add(src)
  popover.value?.close()
}
</script>

<template>
  <KunPopover
    ref="popover"
    position="bottom-start"
    :opaque="true"
    inner-class="w-80 space-y-3 p-3"
  >
    <template #trigger>
      <KunTooltip text="插入图片" :delay-show="300">
        <KunButton
          variant="light"
          size="sm"
          :is-icon-only="true"
          aria-label="插入图片"
        >
          <KunIcon name="lucide:image" />
        </KunButton>
      </KunTooltip>
    </template>

    <!-- Paste a URL -->
    <div class="flex items-center gap-2">
      <KunInput
        v-model="url"
        placeholder="粘贴图片链接…"
        size="sm"
        class-name="flex-1"
        @keydown.enter="insertUrl"
      />
      <KunButton
        size="sm"
        color="primary"
        :disabled="!url.trim()"
        @click="insertUrl"
      >
        插入
      </KunButton>
    </div>

    <!-- Click / drag to upload (one or many) -->
    <EditGalgamePrImageDropzone
      label="点击选择或拖拽图片（可多张）"
      accept="image/*"
      :is-uploading="isUploading"
      :progress-text="progressText"
      @files="uploadFiles"
    />

    <!-- Recently used -->
    <div v-if="history.images.length" class="space-y-1">
      <div class="text-default-500 text-xs">最近使用</div>
      <div class="grid grid-cols-4 gap-1">
        <button
          v-for="src in history.images"
          :key="src"
          type="button"
          class="border-default-200 hover:border-primary-500 aspect-square overflow-hidden rounded border transition-colors"
          @click="insertFromHistory(src)"
        >
          <img
            :src="imageTokenUrl(src)"
            alt=""
            loading="lazy"
            class="h-full w-full object-cover"
          />
        </button>
      </div>
    </div>
  </KunPopover>
</template>
