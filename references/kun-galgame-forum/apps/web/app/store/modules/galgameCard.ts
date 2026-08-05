import { defineStore } from 'pinia'
import { ref } from 'vue'

// Per-user display preferences for the galgame list card, persisted to
// localStorage. Controls which of the four banner corners show, plus the
// NSFW badge, the footer (publisher + time), the secondary Japanese title,
// and whether a card opens in a new tab. Read by
// components/galgame/card/Card.vue; toggled from the "显示设置" panel in
// card/Nav.vue. Defaults reproduce the original always-on layout (corners +
// NSFW badge + footer on; JP name opt-in) and open a clicked card in the
// same tab.
export const usePersistGalgameCardStore = defineStore(
  'KUNGalgameCardDisplay',
  () => {
    const showPlatform = ref(true) // top-left
    const showRating = ref(true) // top-right
    const showViewLike = ref(true) // bottom-left
    const showLanguage = ref(true) // bottom-right
    const showNsfwBadge = ref(true)
    const showPublisher = ref(true)
    const showJapaneseName = ref(false)
    const isOpenInNewTab = ref(false) // click a card → open /galgame/:id (false = same tab; the default)

    const reset = () => {
      showPlatform.value = true
      showRating.value = true
      showViewLike.value = true
      showLanguage.value = true
      showNsfwBadge.value = true
      showPublisher.value = true
      showJapaneseName.value = false
      isOpenInNewTab.value = false
    }

    return {
      showPlatform,
      showRating,
      showViewLike,
      showLanguage,
      showNsfwBadge,
      showPublisher,
      showJapaneseName,
      isOpenInNewTab,
      reset
    }
  },
  {
    persist: true
  }
)
