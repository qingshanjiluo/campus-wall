import { defineStore } from 'pinia'
import { ref } from 'vue'

// A recently-associated galgame remembered for the quiz 出题 picker. We keep the
// resolved display name + banner + 会社 so the LRU quick-picks render the same
// rich row as live search results.
export interface RecentQuizGalgame {
  id: number
  name: string
  banner?: string
  thumbhash?: string
  officials?: string[]
}

const MAX_RECENT = 8

// LRU of galgames the user recently linked to a quiz, so re-associating the same
// game is one click. Persisted per browser; newest first, deduped by id, capped.
// Same shape/idiom as usePersistImageHistoryStore.
export const usePersistQuizGalgameStore = defineStore(
  'KUNGalgameQuizGalgame',
  () => {
    const recent = ref<RecentQuizGalgame[]>([])

    const add = (game: RecentQuizGalgame) => {
      if (!game?.id) {
        return
      }
      recent.value = [
        game,
        ...recent.value.filter((g) => g.id !== game.id)
      ].slice(0, MAX_RECENT)
    }

    const remove = (id: number) => {
      recent.value = recent.value.filter((g) => g.id !== id)
    }

    return { recent, add, remove }
  },
  { persist: { storage: piniaPluginPersistedstate.localStorage() } }
)
