<script setup lang="ts">
import type { KunSelectOption, KunTagInputInvalidReason } from '@kungal/ui-vue'
import {
  kunGalgameToolsetTypeOptions,
  kunGalgameToolsetLanguageOptions,
  kunGalgameToolsetPlatformOptions,
  kunGalgameToolsetVersionOptions
} from '~/constants/toolset'
import { toolsetUpdateForm } from '~/components/toolset/rewriteStore'
import { updateToolsetSchema } from '~/validations/toolset'

const isSubmitting = ref(false)

const onAliasInvalid = (reason: KunTagInputInvalidReason) => {
  if (reason === 'duplicate') useMessage(10505, 'warn')
  else if (reason === 'max-reached') useMessage(10508, 'warn')
}

const handleSubmit = async () => {
  const result = updateToolsetSchema.safeParse(toolsetUpdateForm)
  if (!result.success) {
    const message = JSON.parse(result.error.message)[0]
    useMessage(
      formatKunZodIssue(message),
      'warn'
    )
    return
  }
  if (isSubmitting.value) {
    return
  }

  isSubmitting.value = true
  const res = await kunFetch<string>(
    `/toolset/${toolsetUpdateForm.toolset_id}`,
    {
      method: 'PUT',
      body: toolsetUpdateForm
    }
  )
  isSubmitting.value = false

  // kunFetch resolves to null on failure, having already shown the backend's
  // error toast. Reporting success and navigating away on top of that made a
  // rejected update look like it had gone through — mirror the create form and
  // stay on the page so the edits aren't lost.
  if (!res) {
    return
  }

  useMessage('更新工具信息成功', 'success')
  navigateTo(`/toolset/${toolsetUpdateForm.toolset_id}`)
}

// Blank entries are dropped: ''.split(',') yields [''], and an empty string
// fails the schema's z.url() check, so clearing the link field used to block
// the whole save with a confusing "无效的 URL".
const handleUpdatePageLink = (value: string | number) => {
  toolsetUpdateForm.homepage = value
    .toString()
    .split(',')
    .map((l) => l.trim())
    .filter(Boolean)
}
</script>

<template>
  <div class="space-y-6">
    <KunHeader
      name="编辑工具信息"
      description="更新你发布的 Galgame 工具信息"
    />

    <div class="space-y-2">
      <label class="text-sm font-medium">名称</label>
      <KunInput v-model="toolsetUpdateForm.name" placeholder="工具名称" />
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
      <KunSelect
        v-model="toolsetUpdateForm.type"
        label="工具类型"
        :options="
          kunGalgameToolsetTypeOptions.filter(
            (o) => o.value !== 'all'
          ) as KunSelectOption<
            Exclude<(typeof kunGalgameToolsetTypeOptions)[number]['value'], 'all'>
          >[]
        "
      />
      <KunSelect
        v-model="toolsetUpdateForm.version"
        label="版本"
        :options="
          kunGalgameToolsetVersionOptions.filter(
            (o) => o.value !== 'all'
          ) as KunSelectOption<
            Exclude<
              (typeof kunGalgameToolsetVersionOptions)[number]['value'],
              'all'
            >
          >[]
        "
      />
    </div>

    <div class="space-y-2">
      <div class="text-xl font-medium">简介</div>
      <p class="text-default-500 text-sm">
        请在此处具体说明工具是什么, 以及如何使用该工具, 越详细越好
      </p>
      <KunMilkdownDualEditorProvider
        :value-markdown="toolsetUpdateForm.description"
        @set-markdown="(value) => (toolsetUpdateForm.description = value)"
        language="zh-cn"
      >
        <KunLink target="_blank" to="/doc/create-galgame-toolset">
          发布 Galgame 工具规定
        </KunLink>
      </KunMilkdownDualEditorProvider>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
      <KunSelect
        v-model="toolsetUpdateForm.platform"
        label="平台"
        :options="
          kunGalgameToolsetPlatformOptions.filter(
            (o) => o.value !== 'all'
          ) as KunSelectOption<
            Exclude<
              (typeof kunGalgameToolsetPlatformOptions)[number]['value'],
              'all'
            >
          >[]
        "
      />
      <KunSelect
        v-model="toolsetUpdateForm.language"
        label="语言"
        :options="
          kunGalgameToolsetLanguageOptions.filter(
            (o) => o.value !== 'all'
          ) as KunSelectOption<
            Exclude<
              (typeof kunGalgameToolsetLanguageOptions)[number]['value'],
              'all'
            >
          >[]
        "
      />
    </div>

    <div class="space-y-2">
      <div class="text-sm font-medium">主页 / 下载链接</div>
      <KunTextarea
        :model-value="toolsetUpdateForm.homepage.toString()"
        @update:model-value="handleUpdatePageLink"
        placeholder="如果有多个页面链接, 需要用英语逗号分隔每个链接"
      />
    </div>

    <div class="space-y-2">
      <div class="text-sm font-medium">别名</div>
      <KunTagInput
        v-model="toolsetUpdateForm.aliases"
        :max-tags="17"
        :max-tag-length="500"
        placeholder="输入别名后回车"
        description="按 Enter 添加，最多 17 个"
        color="primary"
        @invalid="onAliasInvalid"
      />
    </div>

    <div class="flex justify-end">
      <KunButton :loading="isSubmitting" @click="handleSubmit">
        保存
      </KunButton>
    </div>
  </div>
</template>
