<script setup lang="ts">
// Single modal for BOTH publishing a new resource AND editing an
// existing one. Replaced the legacy `<GalgameResourcePublish>` flow,
// which threaded state through useTempGalgameResourceStore and a
// nested KunModal in a parent — that triple-hop emit/store chain was
// the source of the recurring `$nuxt of null` crashes on edit-modal
// close (the live diagnosis turned out to be useKunMessage rendering
// MessageContainer without an app context; see
// packages/ui/app/composables/useKunMessage.ts for the actual fix).
//
// Mode is driven by whether `resource` is supplied:
//   • resource present → edit mode (PUT, "保存修改", title 重新编辑)
//   • resource absent  → create mode (POST, "发布", title 发布)
//
// Both modes use the same form / validation / help block, so there's
// no longer a separate Publish.vue file to keep in sync.
import {
  kunGalgameResourceTypeOptions,
  kunGalgameResourceLanguageOptions,
  kunGalgameResourcePlatformOptions,
  KUN_GALGAME_RESOURCE_TYPE_MAP,
  KUN_GALGAME_RESOURCE_LANGUAGE_MAP,
  KUN_GALGAME_RESOURCE_PLATFORM_MAP
} from '~/constants/galgame'
import { checkGalgameResourcePublish } from '../utils/checkGalgameResourcePublish'
import type {
  KunGalgameResourceTypeOptions,
  KunGalgameResourceLanguageOptions,
  KunGalgameResourcePlatformOptions
} from '~/constants/galgame'

const props = defineProps<{
  galgameId: number
  resource?: GalgameResourceDetailLink | null
  refresh: () => void
}>()

const open = defineModel<boolean>({ required: true })

const nuxtApp = useNuxtApp()

const isEditing = computed(() => !!props.resource)

const modalTitle = computed(() =>
  isEditing.value ? '重新编辑资源信息' : '发布 Galgame 资源'
)
const modalSubtitle = computed(() =>
  isEditing.value
    ? '修改链接 / 提取码 / 备注等信息, 保存后立即生效。'
    : '为这部 Galgame 提交一份新的资源链接, 提交后立即对所有用户可见。'
)
const submitLabel = computed(() =>
  isEditing.value ? '保存修改' : '发布资源'
)

interface FormShape {
  type: KunGalgameResourceTypeOptions
  link: string[]
  language: KunGalgameResourceLanguageOptions
  platform: KunGalgameResourcePlatformOptions
  size: string
  code: string
  password: string
  note: string
}

const defaultForm = (): FormShape => ({
  type: 'game',
  link: [],
  language: 'zh-cn',
  platform: 'windows',
  size: '',
  code: '',
  password: '',
  note: ''
})

const snapshotFromResource = (): FormShape => {
  const r = props.resource
  if (!r) return defaultForm()
  return {
    type: r.type as KunGalgameResourceTypeOptions,
    link: [...r.link],
    language: r.language as KunGalgameResourceLanguageOptions,
    platform: r.platform as KunGalgameResourcePlatformOptions,
    size: r.size,
    code: r.code,
    password: r.password,
    note: r.note
  }
}

const form = ref<FormShape>(snapshotFromResource())

// Re-seed every time the modal opens. Without this, closing without
// saving and reopening would carry stale local edits between sessions
// (or stale fields from a previously-edited resource into a new
// create flow).
watch(open, (isOpen) => {
  if (isOpen) form.value = snapshotFromResource()
})

const isSubmitting = ref(false)

const handleSubmit = async () => {
  if (isSubmitting.value) return
  if (!checkGalgameResourcePublish(form.value)) return

  const method = isEditing.value ? 'PUT' : 'POST'
  const body = isEditing.value
    ? {
        ...form.value,
        galgame_id: props.galgameId,
        galgame_resource_id: props.resource!.id
      }
    : { ...form.value, galgame_id: props.galgameId }

  isSubmitting.value = true
  const result = await nuxtApp.runWithContext(() =>
    kunFetch(`/galgame/${props.galgameId}/resource`, { method, body })
  )
  isSubmitting.value = false

  if (result) {
    nuxtApp.runWithContext(() => {
      // Backend message codes: 10549 = 发布成功, 10550 = 更新成功
      useMessage(isEditing.value ? 10550 : 10549, 'success')
      props.refresh()
      open.value = false
    })
  }
}

const handleCancel = () => {
  open.value = false
}

const typeOptions = computed(() =>
  kunGalgameResourceTypeOptions.filter((o) => o.value !== 'all')
)
const languageOptions = computed(() =>
  kunGalgameResourceLanguageOptions.filter((o) => o.value !== 'all')
)
const platformOptions = computed(() =>
  kunGalgameResourcePlatformOptions.filter((o) => o.value !== 'all')
)
</script>

<template>
  <KunModal v-model="open" inner-class-name="max-w-3xl w-[92vw]" :is-dismissable="false">
    <div class="space-y-5">
      <div class="space-y-1">
        <h2 class="text-lg font-semibold">{{ modalTitle }}</h2>
        <p class="text-default-500 text-sm">{{ modalSubtitle }}</p>
      </div>

      <GalgameResourceHelp />

      <KunTextarea
        :model-value="form.link.join(',')"
        @update:model-value="
          (v) =>
            (form.link = String(v)
              .split(',')
              .map((s) => s.trim())
              .filter(Boolean))
        "
        placeholder="资源链接 (网盘 | 磁链 | 网址); 同一资源多链接用英文逗号分隔"
      />

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <KunInput v-model="form.size" placeholder="资源体积 (MB 或 GB)" />
        <KunInput v-model="form.code" placeholder="提取码 (可选)" />
        <KunInput v-model="form.password" placeholder="解压码 (可选)" />
      </div>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <KunSelect
          label="资源类型"
          :model-value="form.type"
          :options="typeOptions"
          @set="(v) => (form.type = v as KunGalgameResourceTypeOptions)"
        >
          <span>{{ KUN_GALGAME_RESOURCE_TYPE_MAP[form.type] }}</span>
        </KunSelect>

        <KunSelect
          label="资源语言"
          :model-value="form.language"
          :options="languageOptions"
          @set="(v) => (form.language = v as KunGalgameResourceLanguageOptions)"
        >
          <span>{{ KUN_GALGAME_RESOURCE_LANGUAGE_MAP[form.language] }}</span>
        </KunSelect>

        <KunSelect
          label="资源平台"
          :model-value="form.platform"
          :options="platformOptions"
          @set="(v) => (form.platform = v as KunGalgameResourcePlatformOptions)"
        >
          <span>{{ KUN_GALGAME_RESOURCE_PLATFORM_MAP[form.platform] }}</span>
        </KunSelect>
      </div>

      <div class="space-y-1">
        <p class="text-default-600 text-sm font-medium">
          资源备注 (可选) — 注意事项 / 介绍 / 作者信息, 支持 Markdown 与图片
        </p>
        <KunMilkdownDualEditorProvider
          :value-markdown="form.note"
          @set-markdown="(v) => (form.note = v)"
        />
      </div>

      <div class="flex justify-end gap-2">
        <KunButton variant="light" color="default" @click="handleCancel">
          取消
        </KunButton>
        <KunButton
          variant="solid"
          color="primary"
          :loading="isSubmitting"
          @click="handleSubmit"
        >
          {{ submitLabel }}
        </KunButton>
      </div>
    </div>
  </KunModal>
</template>
