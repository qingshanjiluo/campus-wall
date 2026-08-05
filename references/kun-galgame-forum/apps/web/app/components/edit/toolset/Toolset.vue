<script setup lang="ts">
import type { KunSelectOption, KunTagInputInvalidReason } from '@kungal/ui-vue'
import {
  kunGalgameToolsetTypeOptions,
  kunGalgameToolsetLanguageOptions,
  kunGalgameToolsetPlatformOptions,
  kunGalgameToolsetVersionOptions
} from '~/constants/toolset'
import { createToolsetSchema } from '~/validations/toolset'
import type { z } from 'zod'

type CreateFormType = z.infer<typeof createToolsetSchema>

const form = reactive<CreateFormType>({
  name: '',
  description: '',
  language: 'zh-cn',
  platform: 'windows',
  type: 'emulator',
  version: 'stable',
  homepage: [] as string[],
  aliases: [] as string[]
})

const onAliasInvalid = (reason: KunTagInputInvalidReason) => {
  if (reason === 'duplicate') useMessage(10505, 'warn')
  else if (reason === 'max-reached') useMessage(10508, 'warn')
}

const isSubmitting = ref(false)

const handleSubmit = async () => {
  const result = createToolsetSchema.safeParse(form)
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
  // POST /toolset returns the full created toolset row (CreatedToolsetResponse
  // = the GalgameToolset model), NOT a bare id. Read `.id` off it — interpolating
  // the whole object into the URL yielded `/toolset/[object Object]`, which the
  // detail page parsed to NaN and rendered as "未找到该工具资源".
  const created = await kunFetch<{ id: number }>('/toolset', {
    method: 'POST',
    body: form
  })
  isSubmitting.value = false

  if (created) {
    useMessage('创建工具成功', 'success')
    navigateTo(`/toolset/${created.id}`)
  }
}

// Blank entries are dropped: ''.split(',') yields [''], and an empty string
// fails the schema's z.url() check, so clearing the link field used to block
// the whole submit with a confusing "无效的 URL".
const handleUpdatePageLink = (value: string | number) => {
  form.homepage = value
    .toString()
    .split(',')
    .map((l) => l.trim())
    .filter(Boolean)
}
</script>

<template>
  <div class="space-y-6">
    <KunHeader
      name="发布 Galgame 工具"
      description="提交与 Galgame 相关的工具，帮助其他用户下载与使用"
    />

    <div class="space-y-2">
      <div class="text-xl font-medium">名称</div>
      <KunInput v-model="form.name" placeholder="工具名称" />
    </div>

    <div class="grid grid-cols-2 gap-3">
      <KunSelect
        v-model="form.type"
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
        v-model="form.version"
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
      <KunSelect
        v-model="form.platform"
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
        v-model="form.language"
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
      <div class="text-xl font-medium">简介</div>
      <p class="text-default-500 text-sm">
        请在此处具体说明工具是什么, 以及如何使用该工具, 越详细越好
      </p>
      <KunMilkdownDualEditorProvider
        :value-markdown="form.description"
        @set-markdown="(value) => (form.description = value)"
        language="zh-cn"
      >
        <KunLink target="_blank" to="/doc/create-galgame-toolset">
          发布 Galgame 工具规定
        </KunLink>
      </KunMilkdownDualEditorProvider>
    </div>

    <div class="space-y-2">
      <div class="text-xl font-medium">主页</div>
      <KunTextarea
        :model-value="form.homepage.toString()"
        @update:model-value="handleUpdatePageLink"
        placeholder="工具的官网, GitHub 仓库等等, 如果有多个链接, 使用英语逗号分隔每个下载链接"
      />
    </div>

    <div class="space-y-2">
      <div class="text-xl font-medium">别名</div>
      <KunTagInput
        v-model="form.aliases"
        :max-tags="17"
        :max-tag-length="500"
        placeholder="输入别名后按下回车添加"
        description="按 Enter 添加，最多 17 个"
        color="primary"
        @invalid="onAliasInvalid"
      />
    </div>

    <div class="flex justify-end">
      <KunButton :loading="isSubmitting" @click="handleSubmit">
        发布
      </KunButton>
    </div>
  </div>
</template>
