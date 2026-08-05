<script setup lang="ts">
import { provideDocEditorContext } from './context'
import type { DocEditorMode, DocEditorForm } from './type'
import { computeReadingMinute } from '~/utils/doc'
const props = withDefaults(
  defineProps<{
    mode: DocEditorMode
    initialArticle?: DocArticleDetail | null
    // When used standalone (the old /edit/doc pages) we navigate to the saved
    // article. When hosted inside the admin modal we emit `saved` instead so
    // the manager can close the modal + refresh its list in place.
    redirectOnSuccess?: boolean
  }>(),
  {
    initialArticle: null,
    redirectOnSuccess: true
  }
)

const emit = defineEmits<{
  saved: [article: DocArticleDetail]
}>()

const isRewriteMode = computed(() => props.mode === 'rewrite')

const [
  { data: categoryResponse },
  { data: tagResponse, refresh: refreshTagResponse }
] = await Promise.all([
  useKunFetch<DocCategoryListResponse>('/doc/category', {
    query: { page: 1, limit: 100, keyword: '' }
  }),
  useKunFetch<DocTagListResponse>('/doc/tag', {
    query: { page: 1, limit: 100, keyword: '' }
  })
])

const categories = ref<DocCategoryItem[]>([])
const tags = ref<DocTagItem[]>([])

watch(
  categoryResponse,
  (response) => {
    categories.value = response?.items ?? []
  },
  { immediate: true }
)

watch(
  tagResponse,
  (response) => {
    tags.value = response?.items ?? []
  },
  { immediate: true }
)

const createDefaultForm = (): DocEditorForm => ({
  article_id: null,
  title: '',
  slug: '',
  description: '',
  banner: '',
  banner_image_hash: '',
  status: 1,
  is_pin: false,
  content_markdown: '',
  category_id: 0,
  tag_ids: []
})

const form = reactive<DocEditorForm>(createDefaultForm())
const isSubmitting = ref(false)
const readingMinute = computed(() =>
  form.content_markdown.trim() ? computeReadingMinute(form.content_markdown) : 0
)

const applyArticleToForm = (article: DocArticleDetail) => {
  form.article_id = article.id
  form.title = article.title
  form.slug = article.slug
  form.description = article.description
  // Keep the legacy URL so submitting an un-migrated doc preserves it; the hash
  // drives the new uploader.
  form.banner = article.banner || ''
  form.banner_image_hash = article.banner_image_hash ?? ''
  form.status = article.status
  form.is_pin = article.is_pin
  form.content_markdown = article.content_markdown
  form.category_id = article.category_id
  form.tag_ids = article.tag_ids ?? []
}

const resetForm = () => {
  if (isRewriteMode.value && props.initialArticle) {
    applyArticleToForm(props.initialArticle)
    return
  }

  Object.assign(form, createDefaultForm())
}

if (isRewriteMode.value && props.initialArticle) {
  applyArticleToForm(props.initialArticle)
}

watch(
  () => props.initialArticle,
  (article) => {
    if (isRewriteMode.value && article) {
      applyArticleToForm(article)
    }
  }
)

const validateForm = () => {
  if (!form.title.trim()) {
    return '请输入标题'
  }
  if (!form.slug.trim()) {
    return '请输入 slug'
  }
  if (!form.description.trim()) {
    return '请输入简介'
  }
  if (!form.content_markdown.trim()) {
    return '请输入正文内容'
  }
  if (!form.category_id) {
    return '请选择文档分类'
  }
  return true
}

const handleSubmit = async () => {
  if (isSubmitting.value) {
    return
  }

  const validation = validateForm()
  if (validation !== true) {
    useMessage(validation, 'warn')
    return
  }

  if (isRewriteMode.value && !form.article_id) {
    useMessage('未找到文档 ID，无法更新', 'error')
    return
  }

  isSubmitting.value = true
  try {
    const normalizedSlug = form.slug.trim()
    const body: Record<string, unknown> = {
      title: form.title.trim(),
      slug: normalizedSlug,
      description: form.description.trim(),
      banner: form.banner.trim(),
      banner_image_hash: form.banner_image_hash,
      status: form.status,
      is_pin: form.is_pin,
      content_markdown: form.content_markdown,
      category_id: form.category_id as number,
      tag_ids: Array.from(new Set(form.tag_ids))
    }

    if (isRewriteMode.value) {
      body.article_id = form.article_id
    }

    const result = await kunFetch<DocArticleDetail>('/doc/article', {
      method: isRewriteMode.value ? 'PUT' : 'POST',
      body
    })

    if (result) {
      useMessage(
        isRewriteMode.value ? '更新文档成功' : '创建文档成功',
        'success'
      )
      applyArticleToForm(result)
      if (props.redirectOnSuccess) {
        await navigateTo(result.path)
      } else {
        emit('saved', result)
      }
    }
  } finally {
    isSubmitting.value = false
  }
}

const refreshTags = async () => {
  await refreshTagResponse()
}

provideDocEditorContext({
  form,
  categories,
  tags,
  mode: props.mode,
  isSubmitting,
  handleSubmit,
  resetForm,
  refreshTags,
  readingMinute,
  initialBannerUrl: props.initialArticle?.banner_url ?? ''
})
</script>

<template>
  <div class="contents">
    <ClientOnly>
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- min-w-0: the editor track sizes from its content (long unbroken
             markdown lines), which would otherwise widen it and squeeze the
             metadata column as the user types. -->
        <div class="order-2 min-w-0 space-y-6 sm:order-1 lg:col-span-1">
          <EditDocMetadataForm />
          <EditDocSubmitActions />
        </div>

        <div class="order-1 min-w-0 space-y-4 sm:order-2 lg:col-span-2">
          <EditDocTitle />
          <EditDocContentEditor />
        </div>
      </div>
    </ClientOnly>
  </div>
</template>
