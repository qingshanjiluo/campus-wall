import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { EditStoreTemp } from '~/store/types/edit/topic'

export const useTempEditStore = defineStore(
  'tempEdit',
  () => {
    const id = ref<EditStoreTemp['id']>(0)
    const title = ref<EditStoreTemp['title']>('')
    const content = ref<EditStoreTemp['content']>('')
    const category = ref<EditStoreTemp['category']>('')
    const section = ref<EditStoreTemp['section']>([])
    const isNSFW = ref<EditStoreTemp['isNSFW']>(false)
    const coverImages = ref<EditStoreTemp['coverImages']>([])
    const isTopicRewriting = ref<EditStoreTemp['isTopicRewriting']>(false)

    const resetRewriteTopicData = () => {
      id.value = 0
      title.value = ''
      content.value = ''
      category.value = ''
      section.value = []
      isNSFW.value = false
      coverImages.value = []
      isTopicRewriting.value = false
    }

    return {
      id,
      title,
      content,
      category,
      section,
      isNSFW,
      coverImages,
      isTopicRewriting,
      resetRewriteTopicData
    }
  },
  {
    persist: false
  }
)
