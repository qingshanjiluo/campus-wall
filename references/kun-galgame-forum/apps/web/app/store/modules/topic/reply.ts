import { defineStore } from 'pinia'
import { reactive, ref } from 'vue'
import type { ReplyStorePersist } from '~/store/types/topic/reply'

export const usePersistKUNGalgameReplyStore = defineStore(
  'KUNGalgameTopicReply',
  () => {
    const mode = ref<ReplyStorePersist['mode']>('preview')
    const replyDraft = reactive<ReplyStorePersist['replyDraft']>({
      mainContent: ''
    })

    const resetReplyDraft = () => {
      replyDraft.mainContent = ''
    }

    return {
      mode,
      replyDraft,
      resetReplyDraft
    }
  },
  {
    persist: true
  }
)
