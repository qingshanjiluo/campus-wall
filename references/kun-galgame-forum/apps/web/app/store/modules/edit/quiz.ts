import { defineStore } from 'pinia'
import { ref } from 'vue'

// The CREATE 出题 draft, persisted to localStorage so a half-written quiz
// survives a refresh / navigation. Shared across every create surface (the
// /edit/galgame/quiz page + the in-context 出题 modals). EDIT mode does NOT use
// this store — editing an existing quiz seeds from the fetched editData and must
// not clobber the create draft. `content` holds the type-specific payload
// (options + answer key) snapshotted from the content editor. The linked-galgame
// picker is intentionally NOT persisted (optional + quick to re-pick).
export const usePersistEditQuizStore = defineStore(
  'KUNGalgameEditQuiz',
  () => {
    const category = ref<QuizCategory>('trivia')
    const type = ref<QuizType>('single')
    const difficulty = ref(3)
    const spoilerLevel = ref<QuizSpoilerLevel>('none')
    const question = ref('')
    const description = ref('')
    const explanation = ref('')
    const showExplanation = ref(false)
    const hideGalgame = ref(false)
    const content = ref<Record<string, unknown>>({})

    const reset = () => {
      category.value = 'trivia'
      type.value = 'single'
      difficulty.value = 3
      spoilerLevel.value = 'none'
      question.value = ''
      description.value = ''
      explanation.value = ''
      showExplanation.value = false
      hideGalgame.value = false
      content.value = {}
    }

    return {
      category,
      type,
      difficulty,
      spoilerLevel,
      question,
      description,
      explanation,
      showExplanation,
      hideGalgame,
      content,
      reset
    }
  },
  {
    persist: {
      storage: piniaPluginPersistedstate.localStorage()
    }
  }
)
