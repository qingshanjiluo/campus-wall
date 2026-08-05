import { defineStore } from 'pinia'
import { ref } from 'vue'

export const usePersistKUNGalgameSearchStore = defineStore(
  'KUNGalgameSearch',
  () => {
    const searchHistory = ref<string[]>([])

    return { searchHistory }
  },
  {
    persist: true
  }
)
