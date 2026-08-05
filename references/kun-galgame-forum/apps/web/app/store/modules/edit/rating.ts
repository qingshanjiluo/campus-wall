import { defineStore } from 'pinia'
import { ref } from 'vue'

export const usePersistEditGalgameRatingStore = defineStore(
  'KUNGalgameEditGalgameRating',
  () => {
    const shortSummary = ref('')

    return { shortSummary }
  },
  {
    persist: {
      storage: piniaPluginPersistedstate.localStorage()
    }
  }
)
