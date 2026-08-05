<script setup lang="ts">
import {
  FRIEND_LINK_CATEGORY_OPTIONS,
  FRIEND_LINK_STATUS_OPTIONS
} from '~/constants/friendLink'

// FriendLink / FriendLinkInput / FriendLinkCategory are auto-imported (shared/types).
const props = defineProps<{
  modelValue: boolean
  initialData?: FriendLink | null
  defaultCategory?: FriendLinkCategory
}>()

const emits = defineEmits<{
  'update:modelValue': [value: boolean]
  submit: [data: FriendLinkInput]
}>()

const isOpen = computed({
  get: () => props.modelValue,
  set: (v) => emits('update:modelValue', v)
})
const isEditing = computed(() => !!props.initialData?.id)

const getInitial = (): FriendLinkInput => {
  const d = props.initialData
  const base: FriendLinkInput = {
    category: d?.category ?? props.defaultCategory ?? 'galgame',
    name: d?.name ?? '',
    link: d?.link ?? '',
    description: d?.description ?? '',
    // Keep the legacy URL so submitting an un-migrated friend preserves it; the
    // hash drives the new uploader.
    banner: d?.banner ?? '',
    banner_image_hash: d?.banner_image_hash ?? '',
    status: d?.status ?? 'normal'
  }
  return d?.id ? { ...base, id: d.id } : base
}

const form = reactive<FriendLinkInput>(getInitial())
watch(
  () => props.modelValue,
  (open) => {
    if (open) Object.assign(form, getInitial())
  }
)

// Resolved CDN url of the edited friend's banner — preview only (empty on create).
const initialBannerUrl = computed(() => props.initialData?.banner_url ?? '')

const handleSubmit = () => {
  if (!form.name.trim()) {
    useMessage('请填写友链名称', 'warn')
    return
  }
  if (!form.link.trim()) {
    useMessage('请填写友链地址', 'warn')
    return
  }
  emits('submit', { ...form })
  isOpen.value = false
}
</script>

<template>
  <KunModal
    :is-dismissable="false"
    v-model="isOpen"
    inner-class-name="max-w-2xl"
  >
    <form @submit.prevent>
      <h2 class="mb-4 text-xl font-bold">
        {{ isEditing ? '编辑友链' : '添加友链' }}
      </h2>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <KunInput v-model="form.name" label="名称" required />
        <KunInput
          v-model="form.link"
          label="链接 (URL)"
          required
          placeholder="https://..."
        />
        <KunSelect
          v-model="form.category"
          label="分类"
          :options="FRIEND_LINK_CATEGORY_OPTIONS"
        />
        <KunSelect
          v-model="form.status"
          label="状态"
          :options="FRIEND_LINK_STATUS_OPTIONS"
        />
        <KunTextarea
          v-model="form.description"
          label="描述"
          auto-grow
          show-char-count
          :maxlength="500"
          class-name="md:col-span-2"
        />

        <div class="md:col-span-2">
          <KunCoverUpload
            v-model="form.banner_image_hash"
            :preview-url="initialBannerUrl"
            label="图标 / Banner"
          />
        </div>
      </div>

      <div class="mt-6 flex justify-end gap-3">
        <KunButton variant="light" color="danger" @click="isOpen = false">
          取消
        </KunButton>
        <KunButton color="primary" @click="handleSubmit">
          {{ isEditing ? '保存' : '添加' }}
        </KunButton>
      </div>
    </form>
  </KunModal>
</template>
