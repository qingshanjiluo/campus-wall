import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { TopicStorePersist } from '~/store/types/topic/topic'

export const usePersistKUNGalgameTopicStore = defineStore(
  'KUNGalgameTopic',
  () => {
    const layout = ref<TopicStorePersist['layout']>('grid')

    return { layout }
  },
  {
    persist: true
  }
)
