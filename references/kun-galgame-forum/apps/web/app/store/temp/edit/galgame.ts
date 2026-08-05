import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { GalgameEditStoreTemp } from '~/store/types/edit/galgame'

export const useTempGalgamePRStore = defineStore(
  'tempGalgamePR',
  () => {
    const galgamePR = ref<GalgameEditStoreTemp[]>([])

    return { galgamePR }
  },
  {
    persist: false
  }
)
