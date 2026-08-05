<script setup lang="ts">
// Avatar upload — in-app multipart to kungal's proxy, which forwards
// to OAuth's POST /auth/me/avatar (docs/oauth/02-user-profile.md).
// OAuth pipes the bytes to image_service, writes the resulting hash
// back to the user row, and returns { hash, url, variant_urls, ... }
// in one round-trip. We surface the new url into the local user
// store so the avatar swaps everywhere without a page reload.
//
// Allowed types are scoped to common image formats — the underlying
// image_service is more permissive, but a UI-side filter keeps users
// out of the "i picked a PDF and got a 400" rabbit hole.
const ACCEPT_TYPES = 'image/png,image/jpeg,image/webp,image/gif,image/avif'
const MAX_BYTES = 4 * 1024 * 1024 // 4 MiB — matches fiber default body cap

const userStore = usePersistUserStore()

const fileInput = ref<HTMLInputElement | null>(null)
const previewUrl = ref<string | null>(null)
const pendingFile = ref<File | null>(null)
const isUploading = ref(false)

interface AvatarUploadResponse {
  hash: string
  url: string
  variant_urls?: Record<string, string>
  width: number
  height: number
  size_bytes: number
  deduplicated: boolean
}

const openPicker = () => fileInput.value?.click()

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  if (!ACCEPT_TYPES.split(',').includes(file.type)) {
    useMessage('请选择 PNG / JPEG / WebP / GIF / AVIF 格式的图片', 'warn')
    target.value = ''
    return
  }
  if (file.size > MAX_BYTES) {
    useMessage('图片不能超过 4 MiB', 'warn')
    target.value = ''
    return
  }

  // Release the previous preview's blob URL before creating a new one
  // so we don't leak object-URL memory across re-pick.
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = URL.createObjectURL(file)
  pendingFile.value = file
}

const clearPick = () => {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = null
  pendingFile.value = null
  if (fileInput.value) fileInput.value.value = ''
}

const submit = async () => {
  if (!pendingFile.value) return
  isUploading.value = true
  const fd = new FormData()
  fd.append('file', pendingFile.value)

  // No `Content-Type` set here on purpose — letting fetch derive it
  // from the FormData preserves the multipart boundary. Setting it
  // manually breaks the upstream binding.
  const result = await kunFetch<AvatarUploadResponse>('/user/avatar', {
    method: 'POST',
    body: fd
  })
  isUploading.value = false

  if (result?.url) {
    useMessage('头像更新成功', 'success')
    // OAuth has already persisted the new avatar; mirror the change
    // into the local store so every component reading user.avatar
    // (top bar, comments, etc.) re-renders without a reload.
    userStore.avatar = result.url
    // withImageVariant handles both image_service (`_100`) and legacy
    // nitro (`-100`) URL conventions automatically; see helper for the
    // detection rule.
    userStore.avatarMin = withImageVariant(result.url, '100')
    clearPick()
  }
}
</script>

<template>
  <KunCard :is-hoverable="false" content-class="space-y-3">
    <div class="space-y-2">
      <span class="text-xl">更改头像</span>
      <p class="text-default-500 text-sm">
        头像统一由 鲲 Galgame OAuth 账户中心管理, 在
        {{ kungal.titleShort }} 直接上传图片即可生效。支持 PNG / JPEG / WebP /
        GIF / AVIF, 不超过 4 MiB。
      </p>
      <p class="text-default-500 text-sm">
        默认头像将会从
        <KunLink size="sm" :to="kungal.domain.sticker" target="_blank">
          鲲 Galgame 表情包
        </KunLink>
        中随机选取, 每一次都是不同的孩子哦, 欸嘿嘿嘿
      </p>
    </div>

    <input
      ref="fileInput"
      type="file"
      :accept="ACCEPT_TYPES"
      class="hidden"
      @change="handleFileChange"
    />

    <div class="flex items-center gap-4">
      <KunAvatar
        :user="{
          id: userStore.id,
          avatar: previewUrl ?? userStore.avatar,
          name: userStore.name
        }"
        size="lg"
        :is-navigation="false"
      />
      <div class="flex-1 text-sm">
        <p v-if="!pendingFile" class="text-default-500">
          当前头像。点击右侧按钮选择新图片。
        </p>
        <p v-else class="text-default-700">
          已选择 <strong>{{ pendingFile.name }}</strong>
          <span class="text-default-400">
            ({{ Math.round(pendingFile.size / 1024) }} KB)
          </span>
        </p>
      </div>
    </div>

    <div class="flex flex-wrap justify-end gap-2">
      <KunButton v-if="pendingFile" variant="light" @click="clearPick">
        取消
      </KunButton>
      <KunButton variant="flat" @click="openPicker">
        {{ pendingFile ? '重新选择' : '选择图片' }}
      </KunButton>
      <KunButton
        :disabled="!pendingFile || isUploading"
        :loading="isUploading"
        @click="submit"
      >
        上传并保存
      </KunButton>
    </div>
  </KunCard>
</template>
