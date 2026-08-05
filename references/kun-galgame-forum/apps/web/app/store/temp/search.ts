import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { SearchStoreTemp } from '../types/search'

export const useTempSearchStore = defineStore(
  'tempSearch',
  () => {
    const keywords = ref<SearchStoreTemp['keywords']>('')

    return { keywords }
  },
  {
    persist: false
  }
)
