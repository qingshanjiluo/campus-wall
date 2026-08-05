import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { MessageStore } from '~/store/types/components/message'

// `alert` used to be a self-contained confirm dialog: this store held
// `showAlert` / `alertTitle` / etc. and some kungal-specific component
// rendered them. After the KunUI layer migration, that renderer is
// gone — nothing reads `showAlert` anymore. Calls to the old
// `.alert(...)` therefore set a ref that NO COMPONENT renders, no UI
// shows up, the returned promise never resolves, and any handler
// awaiting on it hangs silently forever. That's the "毫无反应"
// symptom on every 删除 / 报告失效 / 编辑 button that funnels through
// a confirm step.
//
// Fix: delegate to the layer's useKunAlert (which has a globally
// mounted <KunAlert> + matching state in the layer). Callers keep the
// same `useComponentMessageStore().alert(title, message, showCancel)`
// signature — zero migration churn — but the dialog actually renders
// and resolves now. The legacy fields are left as refs to keep
// `store/index.ts`'s reset (`componentMessageStore.showAlert = false`)
// and any unrelated consumer (isShowCapture / codeSalt) compiling.
export const useComponentMessageStore = defineStore(
  'tempComponentMessage',
  () => {
    const showAlert = ref<MessageStore['showAlert']>(false)
    const alertTitle = ref<MessageStore['alertTitle']>('')
    const alertMsg = ref<MessageStore['alertMsg']>('')
    const isShowCancel = ref<MessageStore['isShowCancel']>(false)
    const isShowCapture = ref<MessageStore['isShowCapture']>(false)
    const isCaptureSuccessful = ref<MessageStore['isCaptureSuccessful']>(false)
    const codeSalt = ref<MessageStore['codeSalt']>('')

    const alert = (
      title?: string,
      message?: string,
      showCancel?: boolean
    ): Promise<boolean> =>
      useKunAlert({
        title,
        message,
        showCancel: showCancel ?? true
      })

    // Stubs kept for backwards-compat with anything that still imports
    // them. The actual confirm/cancel buttons now live inside KunUI's
    // <KunAlert> and call useKunAlertState().handleConfirm / handleCancel.
    const handleClose = () => {}
    const handleConfirm = () => {}

    return {
      showAlert,
      alertTitle,
      alertMsg,
      isShowCancel,
      isShowCapture,
      isCaptureSuccessful,
      codeSalt,
      alert,
      handleClose,
      handleConfirm
    }
  },
  {
    persist: false
  }
)
