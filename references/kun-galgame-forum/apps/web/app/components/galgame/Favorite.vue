<script setup lang="ts">
// Favorite is now membership in one or more collections (收藏夹). The heart is a
// button that opens the collection picker; its filled state reflects whether the
// game is in >=1 of the user's collections. targetUserId is accepted (call sites
// still pass it) but unused — the owner economics live entirely on the server.
const props = defineProps<{
  galgameId: number
  targetUserId?: number
  favoriteCount: number
  isFavorited: boolean
}>()

const { id } = usePersistUserStore()

const isFavorited = ref(props.isFavorited)
const favoriteCount = ref(props.favoriteCount)

// The feed hydrates is-favorited asynchronously (see useMyGalgameInteractions),
// so reflect a late-arriving initial state. Harmless on the detail page.
watch(
  () => props.isFavorited,
  (value) => (isFavorited.value = value)
)
watch(
  () => props.favoriteCount,
  (value) => (favoriteCount.value = value)
)

const pickerOpen = ref(false)

const onClick = () => {
  if (!id) {
    useAuthModal().open()
    return
  }
  pickerOpen.value = true
}

const onSaved = (payload: { favorited: boolean }) => {
  if (isFavorited.value !== payload.favorited) {
    favoriteCount.value += payload.favorited ? 1 : -1
  }
  isFavorited.value = payload.favorited
  // Keep the shared feed state in sync so other cards reflect the change.
  useMyGalgameInteractions().setFavorited(props.galgameId, payload.favorited)
}
</script>

<template>
  <KunTooltip text="收藏">
    <!-- Favorite = collection membership, so this is a controlled action-mode
         KunReaction (:toggle="false"): the filled state reflects "in >=1 收藏夹"
         and the click opens the picker instead of self-toggling. -->
    <span class="flex">
      <KunReaction
        :model-value="isFavorited"
        :count="favoriteCount"
        :toggle="false"
        icon="lucide:heart"
        color="danger"
        label="收藏"
        @click="onClick"
      />
    </span>
  </KunTooltip>

  <GalgameCollectionPickerModal
    v-model="pickerOpen"
    :galgame-id="galgameId"
    @saved="onSaved"
  />
</template>
