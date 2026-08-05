import { useTopicEditorStore } from './useTopicEditorStore'
import type { TopicCategoryKey } from '~/constants/topic'

export interface TopicDraftListItem {
  id: number
  title: string
  // A short raw-markdown prefix of the content, for title-less drafts.
  summary: string
  updated: string
}

interface TopicDraftDetail {
  id: number
  title: string
  content: string
  category: string
  section: string[]
  is_nsfw: boolean
  cover_images: string[]
  updated: string
}

// Server-side, per-user topic drafts (private to the author). The current editor
// state is read from / written back to the shared editor store, so saving a draft
// snapshots exactly what would be published, and loading one restores it fully.
export const useTopicDraft = () => {
  const { title, content, category, section, isNSFW, coverImages } =
    useTopicEditorStore()

  const listDrafts = () =>
    kunFetch<TopicDraftListItem[]>('/topic/draft', { method: 'GET' })

  const saveCurrentAsDraft = () =>
    kunFetch<number>('/topic/draft', {
      method: 'POST',
      body: {
        title: title.value,
        content: content.value,
        category: category.value,
        section: section.value,
        is_nsfw: isNSFW.value,
        cover_images: coverImages.value
      }
    })

  const loadDraft = async (id: number) => {
    const d = await kunFetch<TopicDraftDetail>(`/topic/draft/${id}`, {
      method: 'GET'
    })
    if (!d) {
      return false
    }
    title.value = d.title
    content.value = d.content
    category.value = (d.category as TopicCategoryKey | '') ?? ''
    section.value = d.section ?? []
    isNSFW.value = d.is_nsfw
    coverImages.value = d.cover_images ?? []
    return true
  }

  const deleteDraft = (id: number) =>
    kunFetch<string>(`/topic/draft/${id}`, { method: 'DELETE' })

  const hasCurrentContent = computed(
    () => !!(title.value.trim() || content.value.trim())
  )

  return {
    listDrafts,
    saveCurrentAsDraft,
    loadDraft,
    deleteDraft,
    hasCurrentContent
  }
}
