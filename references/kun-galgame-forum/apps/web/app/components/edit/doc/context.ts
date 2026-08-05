import { inject, provide } from 'vue'
import type { ComputedRef, Ref } from 'vue'
import type { DocEditorForm, DocEditorMode } from './type'

export interface DocEditorContext {
  form: DocEditorForm
  categories: Ref<DocCategoryItem[]>
  tags: Ref<DocTagItem[]>
  mode: DocEditorMode
  isSubmitting: Ref<boolean>
  handleSubmit: () => Promise<void>
  resetForm: () => void
  refreshTags: () => Promise<void>
  readingMinute: ComputedRef<number>
  // Resolved CDN url of the existing cover, for seeding the uploader preview
  // when editing. Preview-only — never the submitted value (that is the hash).
  initialBannerUrl: string
}

const docEditorContextKey = Symbol('DocEditorContext')

export const provideDocEditorContext = (context: DocEditorContext) => {
  provide(docEditorContextKey, context)
}

export const useDocEditorContext = () => {
  const context = inject<DocEditorContext | null>(docEditorContextKey, null)
  if (!context) {
    throw new Error('Doc editor context is not provided')
  }
  return context
}
