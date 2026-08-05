<script setup lang="ts">
// The list of interrupted toolset uploads shown when the upload modal opens.
// Purely presentational + intent-emitting: it owns the per-item file re-pick
// (the browser can't read a file by path, so resuming needs the user to choose
// the file again — matched by size+lastModified, NOT name, so a moved or renamed
// file still resumes) and emits `continue` only on a match, or `delete` to
// discard. The actual resume/abort work lives in Upload.vue, which owns the flow.
defineProps<{
  pending: ToolsetPendingUpload[]
}>()

const emits = defineEmits<{
  continue: [record: ToolsetPendingUpload, file: File]
  delete: [artifactUuid: string]
}>()

const fileInput = ref<HTMLInputElement>()
const pickingFor = ref<ToolsetPendingUpload | null>(null)

const pickFor = (record: ToolsetPendingUpload) => {
  pickingFor.value = record
  fileInput.value?.click()
}

const onPicked = (e: Event) => {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  const record = pickingFor.value
  // Reset so picking the same file again re-fires change.
  input.value = ''
  pickingFor.value = null
  if (!file || !record) {
    return
  }
  // Match on size+lastModified (not name): a moved/renamed file still resumes,
  // and any content edit changes size or mtime, so this still guarantees it's
  // the same file version (avoids stitching a different file's bytes onto the
  // already-uploaded parts).
  if (file.size !== record.size || file.lastModified !== record.last_modified) {
    useMessage('所选文件与未完成的上传不一致, 请选择同一个文件', 'warn')
    return
  }
  emits('continue', record, file)
}
</script>

<template>
  <div class="space-y-2">
    <input ref="fileInput" type="file" hidden @change="onPicked" />

    <div
      v-for="item in pending"
      :key="item.artifact_uuid"
      class="border-default-200 flex flex-col gap-2 rounded-lg border p-3"
    >
      <div class="flex items-center gap-2">
        <KunIcon name="lucide:file-archive" class="text-default-500 shrink-0" />
        <span class="text-default-700 truncate text-sm font-medium">
          {{ item.name }}
        </span>
        <span class="text-default-500 ml-auto shrink-0 text-xs">
          {{ formatFileSize(item.size) }}
        </span>
      </div>

      <KunProgress :value="item.progress" />

      <div class="flex items-center justify-between gap-2">
        <span class="text-default-500 text-xs">
          已上传 {{ item.progress }}%
        </span>
        <div class="flex items-center gap-1">
          <KunButton size="sm" variant="flat" @click="pickFor(item)">
            继续上传
          </KunButton>
          <KunButton
            size="sm"
            variant="light"
            color="danger"
            @click="emits('delete', item.artifact_uuid)"
          >
            彻底删除
          </KunButton>
        </div>
      </div>
    </div>
  </div>
</template>
