<script setup lang="ts">
import { watchDebounced } from '@vueuse/core'

const { searchHistory } = storeToRefs(usePersistKUNGalgameSearchStore())
const { keywords } = storeToRefs(useTempSearchStore())

const isFocus = ref(false)
const input = ref<HTMLElement | null>(null)
// The box is the single source of truth for what the user typed; the shared
// `keywords` (which Container's fetch watcher reads) is DERIVED from it.
const inputValue = ref('')

// Typing → keywords, debounced. We WATCH the v-model ref instead of reading
// the value inside an @input handler: that handler captured the value from
// *before* the keystroke applied, so every search ran the previous term and
// clearing the box still searched the old keyword. Watching the ref always
// publishes the latest text. No maxWait — for a search box it adds a second
// (maxWait) invocation on top of the trailing one, firing each query twice.
watchDebounced(
  () => inputValue.value,
  (value) => {
    keywords.value = value.trim()
  },
  { debounce: 500 }
)

// Reflect EXTERNAL keyword changes (clicking a search-history entry sets the
// store directly) back into the box. The trim guard skips our own debounced
// echo, so it never strips a space the user is still typing. `immediate` so that
// returning to /search (no longer keepalive) restores the persisted keyword into
// the box on mount.
watch(
  keywords,
  (value) => {
    if (value !== inputValue.value.trim()) {
      inputValue.value = value
    }
  },
  { immediate: true }
)

onMounted(() => input.value?.focus())

const handleInputBlur = () => {
  isFocus.value = false
  if (!keywords.value.trim()) {
    return
  }

  if (!searchHistory.value.includes(keywords.value)) {
    searchHistory.value.push(keywords.value)
  }
}

// Enter searches immediately rather than waiting out the debounce window.
const handleEnter = () => {
  keywords.value = inputValue.value.trim()
}
</script>

<template>
  <KunInput
    ref="input"
    v-model="inputValue"
    type="search"
    size="lg"
    :color="isFocus ? 'primary' : 'default'"
    placeholder="输入内容以自动搜索"
    @focus="isFocus = true"
    @blur="handleInputBlur"
    @keydown.enter="handleEnter"
  />
</template>
