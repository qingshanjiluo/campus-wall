import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { TempSettingStore } from '~/store/types/settings'

export const useTempSettingStore = defineStore(
  'tempSetting',
  () => {
    const showKUNGalgameHamburger =
      ref<TempSettingStore['showKUNGalgameHamburger']>(false)
    const showKUNGalgamePanel =
      ref<TempSettingStore['showKUNGalgamePanel']>(false)
    const showKUNGalgameUserPanel =
      ref<TempSettingStore['showKUNGalgameUserPanel']>(false)
    const showKUNGalgameMessageBox =
      ref<TempSettingStore['showKUNGalgameMessageBox']>(false)
    const showKUNGalgameMoemoepointLog =
      ref<TempSettingStore['showKUNGalgameMoemoepointLog']>(false)
    const showKUNGalgameLogout =
      ref<TempSettingStore['showKUNGalgameLogout']>(false)
    const showKUNGalgameCreatorApply =
      ref<TempSettingStore['showKUNGalgameCreatorApply']>(false)
    const messageStatus = ref<TempSettingStore['messageStatus']>('offline')

    const reset = () => {
      showKUNGalgameHamburger.value = false
      showKUNGalgamePanel.value = false
      showKUNGalgameUserPanel.value = false
      showKUNGalgameMessageBox.value = false
      showKUNGalgameMoemoepointLog.value = false
      showKUNGalgameLogout.value = false
      showKUNGalgameCreatorApply.value = false
    }

    return {
      showKUNGalgameHamburger,
      showKUNGalgamePanel,
      showKUNGalgameUserPanel,
      showKUNGalgameMessageBox,
      showKUNGalgameMoemoepointLog,
      showKUNGalgameLogout,
      showKUNGalgameCreatorApply,
      messageStatus,
      reset
    }
  },
  {
    persist: false
  }
)
