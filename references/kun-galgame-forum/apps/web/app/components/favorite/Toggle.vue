<script setup lang="ts">
// The one shared 收藏 toggle: a KunReaction pill wrapping the optimistic
// flip → PUT → revert-on-failure logic that every simple favorite reused. Point
// it at a domain endpoint and it's done.
//
// NOTE: galgame favorites are NOT a toggle — they're membership in a 收藏夹
// (collection picker), so they use a bare KunReaction with :toggle="false" that
// opens the picker instead of this component (see galgame/Favorite.vue).
const props = withDefaults(
  defineProps<{
    favorited: boolean
    count: number
    // The PUT endpoint that toggles the favorite.
    endpoint: string
    // Optional PUT body (some domains want the id echoed in the body).
    body?: Record<string, unknown>
    // [on, off] success feedback — a kunMessage code or a literal string.
    messages?: [string | number, string | number]
    label?: string
    tooltip?: string
    size?: 'sm' | 'md' | 'lg'
  }>(),
  { label: '收藏', tooltip: '收藏', size: 'md', body: undefined, messages: undefined }
)

const emits = defineEmits<{ changed: [favorited: boolean] }>()

const { id } = usePersistUserStore()
const isFavorited = ref(props.favorited)
const favoriteCount = ref(props.count)
const pending = ref(false)

// Feed cards hydrate is-favorited asynchronously → reflect a late initial state.
watch(
  () => props.favorited,
  (v) => (isFavorited.value = v)
)
watch(
  () => props.count,
  (v) => (favoriteCount.value = v)
)

// KunReaction flips the model + count optimistically before @change fires; we
// hit the API and undo on failure / when signed out.
const revert = (next: boolean) => {
  isFavorited.value = !next
  favoriteCount.value += next ? -1 : 1
}

const onChange = async (next: boolean) => {
  if (!id) {
    useAuthModal().open()
    revert(next)
    return
  }
  pending.value = true
  const result = await kunFetch(props.endpoint, {
    method: 'PUT',
    ...(props.body ? { body: props.body } : {})
  })
  pending.value = false
  if (!result) {
    revert(next)
    return
  }
  if (props.messages) {
    useMessage(next ? props.messages[0] : props.messages[1], 'success')
  }
  emits('changed', next)
}
</script>

<template>
  <KunTooltip :text="tooltip">
    <!-- flex span removes the inline line-box so the icon + count sit level with
         any reaction/button beside it (mirrors reaction/Trigger.vue). -->
    <span class="flex">
      <KunReaction
        v-model="isFavorited"
        v-model:count="favoriteCount"
        :disabled="pending"
        :size="size"
        icon="lucide:heart"
        color="danger"
        :label="label"
        @change="onChange"
      />
    </span>
  </KunTooltip>
</template>
