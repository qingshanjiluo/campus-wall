import { defineStore } from 'pinia'
import { ref } from 'vue'

interface CategoryStorePersist {
  category: 'galgame' | 'technique' | 'others'
}

export const usePersistCategoryStore = defineStore(
  'KUNGalgameCategory',
  () => {
    const category = ref<CategoryStorePersist['category']>('galgame')

    return { category }
  },
  {
    persist: true
  }
)
