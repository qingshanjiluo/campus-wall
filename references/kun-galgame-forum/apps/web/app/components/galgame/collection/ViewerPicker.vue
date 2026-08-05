<script setup lang="ts">
import { watchDebounced } from '@vueuse/core'

// Restricted-visibility allow-list picker. Reuses OAuth /user/search (the same
// endpoint the @-mention autocomplete uses); results are never cached.
const props = defineProps<{
  modelValue: CollectionUserBrief[]
}>()

const emits = defineEmits<{
  'update:modelValue': [value: CollectionUserBrief[]]
}>()

const selected = computed({
  get: () => props.modelValue,
  set: (value) => emits('update:modelValue', value)
})

const keyword = ref('')
const results = ref<CollectionUserBrief[]>([])
const searching = ref(false)

watchDebounced(
  keyword,
  async (value) => {
    const q = value.trim()
    if (!q) {
      results.value = []
      return
    }
    searching.value = true
    const res = await kunFetch<CollectionUserBrief[]>('/user/search', {
      query: { q, limit: 8 }
    })
    searching.value = false
    results.value = res ?? []
  },
  { debounce: 300, maxWait: 1000 }
)

const add = (user: CollectionUserBrief) => {
  if (selected.value.some((s) => s.id === user.id)) {
    return
  }
  selected.value = [
    ...selected.value,
    { id: user.id, name: user.name, avatar: user.avatar }
  ]
  keyword.value = ''
  results.value = []
}

const remove = (id: number) => {
  selected.value = selected.value.filter((s) => s.id !== id)
}
</script>

<template>
  <div class="space-y-2">
    <span class="text-sm font-medium">指定可见用户</span>

    <div v-if="selected.length" class="flex flex-wrap gap-2">
      <span
        v-for="u in selected"
        :key="u.id"
        class="bg-default-100 flex items-center gap-1 rounded-full py-1 pr-2 pl-1 text-sm"
      >
        <img
          :src="u.avatar"
          :alt="u.name"
          class="size-5 rounded-full object-cover"
        />
        {{ u.name }}
        <button
          type="button"
          class="text-default-400 hover:text-danger"
          @click="remove(u.id)"
        >
          <KunIcon name="lucide:x" />
        </button>
      </span>
    </div>

    <KunInput v-model="keyword" placeholder="搜索用户名以添加..." />

    <div
      v-if="results.length"
      class="border-default-200 max-h-48 overflow-y-auto rounded-lg border"
    >
      <button
        v-for="u in results"
        :key="u.id"
        type="button"
        class="hover:bg-default-100 flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors"
        @click="add(u)"
      >
        <img
          :src="u.avatar"
          :alt="u.name"
          class="size-6 rounded-full object-cover"
        />
        {{ u.name }}
      </button>
    </div>
  </div>
</template>
