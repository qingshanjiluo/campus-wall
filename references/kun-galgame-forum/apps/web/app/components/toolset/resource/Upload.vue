<script setup lang="ts">
import {
  MAX_SMALL_FILE_SIZE,
  MAX_LARGE_FILE_SIZE,
  USER_DAILY_UPLOAD_LIMIT,
  MOEMOEPOINT_SINGLE_MB_DIVISOR
} from '~/config/upload'
import {
  initToolsetUploadSchema,
  completeToolsetUploadSchema,
  resumeToolsetUploadSchema,
  abortToolsetUploadSchema
} from '~/validations/toolset'
import {
  KUN_GALGAME_TOOLSET_UPLOAD_STATUS_MAP,
  type KUN_GALGAME_TOOLSET_UPLOAD_STATUS_CONST
} from '~/constants/toolset'

const props = defineProps<{
  toolsetId: number
}>()

const emits = defineEmits<{
  onUploadSuccess: [ToolsetUploadResult]
  onClose: []
}>()

const MB = 1024 * 1024
// Fallback chunk size for pre-flight math only; the artifact service decides the
// authoritative slicing and returns part_size in its init response.
const LARGE_CHUNK_SIZE = 5 * MB
const UPLOAD_TRANSFER_FAILED = 'UPLOAD_TRANSFER_FAILED'
const DEFAULT_BINARY_CONTENT_TYPE = 'application/octet-stream'
type ToolsetUploadStatus =
  (typeof KUN_GALGAME_TOOLSET_UPLOAD_STATUS_CONST)[number]
type ToolsetUploadPart = {
  part_number: number
  etag: string
}

// Browsers don't always populate File.type for archive formats (.7z, .rar
// in particular often come back empty). Fall back to a generic binary
// content-type so the API's required-field validator passes and so the
// presigned PUT later sets a sensible Content-Type header on S3.
const resolveContentType = (file: File): string =>
  file.type && file.type.length > 0 ? file.type : DEFAULT_BINARY_CONTENT_TYPE

const { moemoepoint, dailyToolsetUploadBytes } = storeToRefs(
  usePersistUserStore()
)
// Upload bypass: hold this permission to skip the per-user daily quota / single-
// file cap (moderator+). Owner/quota checks otherwise apply.
const canUploadBypass = useCan('toolset.upload_bypass')
const fileInput = ref<HTMLInputElement>()
const selectedFile = ref<File | null>(null)

const progress = ref(0)
const isDragging = ref(false)
const uploadStatus = ref<ToolsetUploadStatus>('idle')
// Set when the selected file matches an interrupted multipart upload — persisted
// across a page refresh, or failed mid-flight this session. While set, submit
// RESUMES (skips parts already in B2) instead of starting over. Cleared on a
// fresh select / successful complete / abort.
const resumeUuid = ref<string | null>(null)

const isLarge = computed(() => {
  const f = selectedFile.value
  return !!f && f.size > MAX_SMALL_FILE_SIZE
})
const dailyUploadLimit = computed(() => {
  if (canUploadBypass.value) {
    return MAX_LARGE_FILE_SIZE
  }

  // Remaining daily budget = (100MB + moemoepoint·MB) − bytes already used today.
  return Math.max(
    0,
    USER_DAILY_UPLOAD_LIMIT +
      moemoepoint.value * MB -
      dailyToolsetUploadBytes.value
  )
})
const maxSingleFileLimit = computed(() => {
  if (canUploadBypass.value) {
    return MAX_LARGE_FILE_SIZE
  }

  const moemoepointMaxSingleFile =
    Math.floor(moemoepoint.value / MOEMOEPOINT_SINGLE_MB_DIVISOR) * MB

  return Math.min(
    Math.max(USER_DAILY_UPLOAD_LIMIT, moemoepointMaxSingleFile),
    MAX_LARGE_FILE_SIZE
  )
})

const statusMessage = computed(() => {
  if (uploadStatus.value === 'largeUploading') {
    return `正在上传大文件【进度 ${progress.value}%】`
  } else {
    return KUN_GALGAME_TOOLSET_UPLOAD_STATUS_MAP[uploadStatus.value]
  }
})

const resetUploadState = () => {
  progress.value = 0
  uploadStatus.value = 'idle'
}

const setSelectedUploadFile = (file: File) => {
  selectedFile.value = file
  resetUploadState()
  // Re-selecting the file of an interrupted upload offers to resume from the
  // breakpoint. Matched by size+lastModified (NOT name) so a moved OR renamed
  // file still resumes; any content edit changes size/mtime, so it still
  // guarantees the same file version.
  const match = resumeStore
    .list()
    .find((p) => p.size === file.size && p.last_modified === file.lastModified)
  resumeUuid.value = match ? match.artifact_uuid : null
}

const throwIfUploadFailed = (response: Response) => {
  if (!response.ok) {
    throw new Error(UPLOAD_TRANSFER_FAILED)
  }
}

const isUploadTransferFailedError = (error: unknown) => {
  return error instanceof Error && error.message === UPLOAD_TRANSFER_FAILED
}

const notifyUploadTransferError = (error: unknown) => {
  if (isUploadTransferFailedError(error)) {
    useMessage('文件传输失败，请重试', 'error')
  }
}

const abortUpload = async (artifactUuid: string) => {
  const abortUploadData = { artifact_uuid: artifactUuid }
  if (!useKunSchemaValidator(abortToolsetUploadSchema, abortUploadData)) {
    return
  }
  try {
    await kunFetch(`/toolset/${props.toolsetId}/upload/abort`, {
      method: 'POST',
      body: abortUploadData
    })
  } catch (abortError) {
    console.error('Failed to abort toolset upload:', abortError)
  }
}

const checkFileValid = (file: File | null) => {
  if (!file) {
    return false
  }
  if (!isValidArchive(file.name || '')) {
    useMessage('我们仅支持 .7z, .zip, .rar 压缩格式上传', 'warn')
    return false
  }
  if (file.size > MAX_LARGE_FILE_SIZE) {
    useMessage(
      `文件大小超过最大文件限制 ${MAX_LARGE_FILE_SIZE / MB} MB`,
      'warn'
    )
    return false
  }
  if (file.size > dailyUploadLimit.value) {
    useMessage(
      `超出当日可用上传额度, 剩余 ${(dailyUploadLimit.value / MB).toFixed(2)} MB`,
      'warn'
    )
    return false
  }
  if (file.size > maxSingleFileLimit.value) {
    useMessage(
      `单文件大小超过限制, 最大 ${(maxSingleFileLimit.value / MB).toFixed(2)} MB`,
      'warn'
    )
    return false
  }
  return true
}

const pick = () => fileInput.value?.click()
const onChange = (e: Event) => {
  const t = e.target as HTMLInputElement
  const targetFile = t.files && t.files[0] ? t.files[0] : null
  const res = checkFileValid(targetFile)
  if (!res) {
    return
  }
  if (!targetFile) {
    return
  }
  setSelectedUploadFile(targetFile)
}
const onDrop = (e: DragEvent) => {
  e.preventDefault()
  e.stopPropagation()
  isDragging.value = false
  const dt = e.dataTransfer
  if (dt?.files && dt.files.length > 0) {
    const res = checkFileValid(dt.files[0]!)
    if (!res) {
      return
    }
    setSelectedUploadFile(dt.files[0]!)
  }
}
const onDragOver = (e: DragEvent) => {
  e.preventDefault()
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = 'copy'
  }
}
const onDragEnter = () => {
  isDragging.value = true
}
const onDragLeave = () => {
  isDragging.value = false
}
const clearSelected = () => {
  selectedFile.value = null
  if (fileInput.value) {
    fileInput.value.value = ''
  }
  resetUploadState()
  // Drop the in-memory resume offer; the persisted record stays so re-selecting
  // the same file can still continue the interrupted upload.
  resumeUuid.value = null
}

// Interrupted multipart uploads are recoverable: the artifact's uploaded parts
// live in B2, so we persist {uuid + file identity + progress} per toolset (via
// useToolsetResumeUploads) and a resume only re-sends the missing parts. The
// browser can't read a file by path, so resuming needs the user to re-select it
// (matched by size+lastModified) — surfaced both by re-dragging it here and by
// the on-open <ToolsetResourceResumeList>.
const resumeStore = useToolsetResumeUploads(props.toolsetId)
const pending = ref<ToolsetPendingUpload[]>([])

const refreshPending = () => {
  pending.value = resumeStore.list()
}

// Remember (insert / replace) an in-flight multipart upload so it can be resumed.
const rememberPending = (f: File, artifactUuid: string, prog: number) => {
  resumeStore.upsert({
    artifact_uuid: artifactUuid,
    name: f.name,
    size: f.size,
    last_modified: f.lastModified,
    progress: prog,
    updated_at: Date.now()
  })
}

// Forget a finished / aborted upload and refresh the banner list.
const forgetPending = (artifactUuid: string) => {
  resumeStore.remove(artifactUuid)
  if (resumeUuid.value === artifactUuid) {
    resumeUuid.value = null
  }
  refreshPending()
}

// PUT a set of parts (slices of f by partSize), collecting their ETags and
// advancing progress against the WHOLE file (doneCount = parts already stored,
// so a resumed upload's bar starts where it left off).
const putParts = async (
  artifactUuid: string,
  f: File,
  partList: { part_number: number; url: string }[],
  partSize: number,
  contentType: string,
  totalParts: number,
  doneCount: number
): Promise<ToolsetUploadPart[]> => {
  const out: ToolsetUploadPart[] = []
  for (let i = 0; i < partList.length; i++) {
    const cur = partList[i]
    if (!cur) {
      throw new Error('Missing upload part')
    }
    const start = (cur.part_number - 1) * partSize
    const end = Math.min(start + partSize, f.size)
    const resp = await fetch(cur.url, {
      headers: { 'Content-Type': contentType },
      method: 'PUT',
      body: f.slice(start, end)
    })
    throwIfUploadFailed(resp)
    const etag = resp.headers.get('ETag') || resp.headers.get('etag')
    if (!etag) {
      throw new Error('Missing ETag')
    }
    out.push({ part_number: cur.part_number, etag })
    progress.value = Math.round(((doneCount + i + 1) / totalParts) * 100)
    // Persist after each part so the resume list stays accurate even if the tab
    // is closed mid-upload (no interruption event fires then).
    resumeStore.setProgress(artifactUuid, progress.value)
  }
  return out
}

// Finalize with kungal; on success emit the result and drop the resume record.
const completeUpload = async (
  artifactUuid: string,
  parts: ToolsetUploadPart[] | undefined
): Promise<boolean> => {
  const completeData = {
    artifact_uuid: artifactUuid,
    parts: parts && parts.length ? parts : undefined
  }
  if (!useKunSchemaValidator(completeToolsetUploadSchema, completeData)) {
    return false
  }
  const done = await kunFetch<ToolsetUploadCompleteResponse>(
    `/toolset/${props.toolsetId}/upload/complete`,
    { method: 'POST', body: completeData }
  )
  if (!done) {
    return false
  }
  useMessage('上传成功', 'success')
  emits('onUploadSuccess', {
    artifact_uuid: done.artifact_uuid,
    size: done.size
  })
  progress.value = 100
  uploadStatus.value = 'complete'
  forgetPending(artifactUuid)
  return true
}

// Park an interrupted multipart upload as resumable: its uploaded parts stay in
// B2, so the user continues from the breakpoint (re-PUTting only the rest) by
// clicking 继续上传 — no abort, no re-sending bytes. Reused for a failed complete
// too: resuming then re-attempts complete (parts already all present).
const registerResumable = (artifactUuid: string, f: File) => {
  rememberPending(f, artifactUuid, progress.value)
  resumeUuid.value = artifactUuid
  uploadStatus.value = 'idle'
  refreshPending()
}

// One server-driven flow: init → PUT (single or multipart, per the init
// response) → complete. Bytes go straight to B2 via the presigned URLs the
// artifact service returns; kungal only brokers the JSON calls. A multipart
// upload is registered resumable on init so an interruption continues from the
// breakpoint (resumeUploadToArtifact) instead of restarting.
const uploadToArtifact = async (f: File) => {
  const contentType = resolveContentType(f)
  const initData = {
    toolset_id: props.toolsetId,
    filename: f.name,
    filesize: f.size,
    content_type: contentType
  }
  if (!useKunSchemaValidator(initToolsetUploadSchema, initData)) {
    return
  }

  progress.value = 0
  uploadStatus.value = isLarge.value ? 'largeInit' : 'smallInit'
  const init = await kunFetch<ToolsetUploadInitResponse>(
    `/toolset/${props.toolsetId}/upload/init`,
    { method: 'POST', body: initData }
  )
  if (!init) {
    uploadStatus.value = 'idle'
    return
  }

  if (init.multipart) {
    // Persist BEFORE the first PUT so even an immediate failure is resumable.
    rememberPending(f, init.artifact_uuid, 0)
    refreshPending()
    const partList = init.parts ?? []
    const partSize = init.part_size || LARGE_CHUNK_SIZE
    try {
      uploadStatus.value = 'largeUploading'
      const parts = await putParts(
        init.artifact_uuid,
        f,
        partList,
        partSize,
        contentType,
        partList.length,
        0
      )
      uploadStatus.value = 'largeComplete'
      if (!(await completeUpload(init.artifact_uuid, parts))) {
        registerResumable(init.artifact_uuid, f)
      }
    } catch (error) {
      // Keep the artifact — its uploaded parts stay in B2 for resume.
      registerResumable(init.artifact_uuid, f)
      notifyUploadTransferError(error)
    }
    return
  }

  // Single-part (small file): no resume bookkeeping — a failure just re-inits.
  try {
    uploadStatus.value = 'smallUploading'
    if (!init.upload_url) {
      throw new Error('Missing upload URL')
    }
    const resp = await fetch(init.upload_url, {
      headers: { 'Content-Type': contentType },
      method: 'PUT',
      body: f
    })
    throwIfUploadFailed(resp)
    uploadStatus.value = 'smallComplete'
    progress.value = 100
    if (!(await completeUpload(init.artifact_uuid, undefined))) {
      resetUploadState()
    }
  } catch (error) {
    await abortUpload(init.artifact_uuid)
    notifyUploadTransferError(error)
    resetUploadState()
  }
}

// Resume an interrupted upload: ask kungal which parts are already stored (skip
// them, reuse their ETags) and PUT only the missing ones, then complete with the
// full set. Falls back to a fresh upload if the session is gone (GC'd / already
// completed / expired → resume 404s).
const resumeUploadToArtifact = async (f: File, artifactUuid: string) => {
  const contentType = resolveContentType(f)
  const resumeData = { artifact_uuid: artifactUuid }
  if (!useKunSchemaValidator(resumeToolsetUploadSchema, resumeData)) {
    return
  }

  progress.value = 0
  uploadStatus.value = 'largeInit'
  const resume = await kunFetch<ToolsetUploadResumeResponse>(
    `/toolset/${props.toolsetId}/upload/resume`,
    { method: 'POST', body: resumeData }
  )
  if (!resume) {
    // No longer resumable (GC'd / completed / expired) — drop the stale record
    // and start fresh.
    forgetPending(artifactUuid)
    await uploadToArtifact(f)
    return
  }

  // Single-part resume = re-PUT the whole file to the fresh URL.
  if (!resume.multipart) {
    try {
      uploadStatus.value = 'smallUploading'
      if (!resume.upload_url) {
        throw new Error('Missing upload URL')
      }
      const resp = await fetch(resume.upload_url, {
        headers: { 'Content-Type': contentType },
        method: 'PUT',
        body: f
      })
      throwIfUploadFailed(resp)
      progress.value = 100
      if (!(await completeUpload(artifactUuid, undefined))) {
        registerResumable(artifactUuid, f)
      }
    } catch (error) {
      registerResumable(artifactUuid, f)
      notifyUploadTransferError(error)
    }
    return
  }

  const partSize = resume.part_size || LARGE_CHUNK_SIZE
  const uploaded = resume.uploaded_parts ?? []
  const missing = resume.parts ?? []
  const totalParts = uploaded.length + missing.length
  try {
    uploadStatus.value = 'largeUploading'
    progress.value =
      totalParts > 0 ? Math.round((uploaded.length / totalParts) * 100) : 0
    const fresh = await putParts(
      artifactUuid,
      f,
      missing,
      partSize,
      contentType,
      totalParts,
      uploaded.length
    )
    const parts: ToolsetUploadPart[] = [
      ...uploaded.map((p) => ({ part_number: p.part_number, etag: p.etag })),
      ...fresh
    ].sort((a, b) => a.part_number - b.part_number)
    uploadStatus.value = 'largeComplete'
    if (!(await completeUpload(artifactUuid, parts))) {
      registerResumable(artifactUuid, f)
    }
  } catch (error) {
    registerResumable(artifactUuid, f)
    notifyUploadTransferError(error)
  }
}

const submit = async () => {
  const f = selectedFile.value
  if (!f) {
    useMessage('请选择文件', 'warn')
    return
  }
  if (resumeUuid.value) {
    await resumeUploadToArtifact(f, resumeUuid.value)
  } else {
    await uploadToArtifact(f)
  }
}

// From the on-open resume list: the user re-picked the matching file (validated
// in ResumeList), so select it and continue from the breakpoint.
const handleContinuePending = (record: ToolsetPendingUpload, file: File) => {
  setSelectedUploadFile(file)
  resumeUploadToArtifact(file, record.artifact_uuid)
}

// Discard an unfinished upload: soft-delete the artifact (its B2 parts are
// reclaimed by GC) and drop the local record.
const handleDeletePending = async (artifactUuid: string) => {
  await abortUpload(artifactUuid)
  forgetPending(artifactUuid)
}

onMounted(refreshPending)
</script>

<template>
  <div class="space-y-4">
    <input ref="fileInput" type="file" hidden @change="onChange" />

    <!-- Surfaced on open: any uploads started-but-not-finished for this toolset
         (persisted across reloads). Resuming needs the user to re-pick the file
         (the browser can't read it by path), so the hint spells that out and the
         list is shown directly. -->
    <div v-if="pending.length && uploadStatus === 'idle'" class="space-y-2">
      <div
        class="text-warning border-warning/30 bg-warning/10 flex items-start gap-2 rounded-lg border p-3 text-sm"
      >
        <KunIcon name="lucide:history" class="mt-0.5 shrink-0" />
        <span>
          您有 {{ pending.length }}
          个未完成的上传，您可以点击「继续上传」后选择未传完的文件继续上传
        </span>
      </div>

      <ToolsetResourceResumeList
        :pending="pending"
        @continue="handleContinuePending"
        @delete="handleDeletePending"
      />
    </div>

    <KunCard :is-hoverable="false" :is-transparent="true">
      <div
        class="cursor-pointer rounded-lg border-2 border-dashed p-6 text-center transition-colors"
        :class="
          cn(
            isDragging
              ? 'border-primary-500 bg-primary-50/50'
              : 'border-default-300 hover:border-default-500'
          )
        "
        @click="pick"
        @drop="onDrop"
        @dragover="onDragOver"
        @dragenter="onDragEnter"
        @dragleave="onDragLeave"
      >
        <div v-if="!selectedFile" class="flex flex-col items-center gap-2">
          <KunIcon
            name="lucide:upload-cloud"
            class="text-default-500 text-3xl"
          />
          <div class="text-default-600">点击或拖拽文件到此处</div>
        </div>

        <div v-else class="flex flex-col justify-center gap-2">
          <div class="flex items-center gap-3">
            <KunIcon
              name="lucide:file-check"
              class="text-success-600 text-xl"
            />
            <div class="text-default-700 font-medium">
              {{ selectedFile?.name }}
            </div>
          </div>

          <div class="flex items-center gap-3">
            <div class="text-default-500 text-xs">
              {{ formatFileSize(selectedFile!.size) }}
            </div>
            <span
              class="border-default-200 bg-default-100 text-default-600 rounded-full border px-2 py-0.5 text-xs"
            >
              {{
                isLarge
                  ? `文件大于 ${MAX_SMALL_FILE_SIZE / MB}MB, 分片上传`
                  : `文件小于 ${MAX_SMALL_FILE_SIZE / MB}MB, 直接上传`
              }}
            </span>
          </div>

          <KunProgress :value="progress" />

          <div
            v-if="resumeUuid && uploadStatus === 'idle'"
            class="text-warning flex items-center justify-center gap-1.5 text-center text-xs"
          >
            <KunIcon name="lucide:history" />
            检测到未完成的上传，将从断点继续
          </div>

          <div
            class="text-default-500 flex items-center justify-center gap-2 text-sm"
          >
            <span>{{ statusMessage }}</span>
            <KunIcon
              class="text-sm"
              v-if="uploadStatus !== 'idle' && uploadStatus !== 'complete'"
              name="svg-spinners:90-ring-with-bg"
            />
            <KunIcon
              class="text-success-600 text-sm"
              v-if="uploadStatus === 'complete'"
              name="lucide:circle-check-big"
            />
          </div>
        </div>
      </div>
    </KunCard>

    <div class="flex items-center justify-end gap-2">
      <KunButton
        v-if="selectedFile"
        variant="light"
        color="danger"
        @click.stop="clearSelected"
      >
        移除文件
      </KunButton>
      <KunButton
        :loading="uploadStatus !== 'idle' && uploadStatus !== 'complete'"
        :disabled="!selectedFile || uploadStatus === 'complete'"
        @click="submit"
      >
        {{ resumeUuid ? '继续上传' : '确认上传' }}
      </KunButton>
    </div>
  </div>
</template>
