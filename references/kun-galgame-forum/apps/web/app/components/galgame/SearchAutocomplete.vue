<script setup lang="ts">
// Shared galgame search picker built on KunAutocomplete (2.11+ async @search /
// :loading / :debounce, 2.12 generic options + #option slot). Searches the wiki
// Meilisearch galgame index — the SAME accurate, ranked search the /search page
// uses — and renders banner + 会社 rows. Emits the picked galgame; the PARENT
// owns the selected-list / chips UI (quiz picker, series picker).
//
// Backed by GET /galgame/search/picker (a generic lightweight galgame search:
// id + name + banner + 会社). The @select payload shape is
// { id, name, banner?, thumbhash?, officials? }.
type GalgameSearchHit = {
  id: number
  name: string
  banner?: string
  thumbhash?: string
  officials?: string[]
}

type AutoOption = GalgameSearchHit & { value: string; label: string }

const props = defineProps<{
  // Hide galgames already selected by the parent.
  excludeIds?: number[]
  placeholder?: string
}>()
const emits = defineEmits<{ select: [hit: GalgameSearchHit] }>()

const query = ref('')
const options = ref<AutoOption[]>([])
const isLoading = ref(false)

// Request versioning. Typing then deleting used to let a slow in-flight fetch
// resolve AFTER the cleared/newer query and clobber the list — a stale-response
// RACE in this handler, NOT a KunAutocomplete debounce bug (its debounce is a
// clean trailing timer). We stamp each search and drop results a newer one
// superseded.
let searchSeq = 0

const onSearch = async (raw: string) => {
  const kw = raw.trim()
  const seq = ++searchSeq
  if (!kw) {
    options.value = []
    isLoading.value = false
    return
  }
  isLoading.value = true
  const data = await kunFetch<QuizGalgameOption[]>('/galgame/search/picker', {
    method: 'GET',
    query: { keywords: kw }
  })
  if (seq !== searchSeq) return // superseded by a newer search — drop stale hits
  const exclude = new Set(props.excludeIds ?? [])
  options.value = (data ?? [])
    .filter((o) => !exclude.has(o.id))
    .map((o) => {
      const name = getPreferredLanguageText(o.name) || `#${o.id}`
      return {
        id: o.id,
        name,
        banner: o.banner,
        thumbhash: o.banner_thumbhash,
        officials: o.officials,
        value: String(o.id),
        label: name
      }
    })
  isLoading.value = false
}

const onSelect = (opt: AutoOption) => {
  emits('select', {
    id: opt.id,
    name: opt.name,
    banner: opt.banner,
    thumbhash: opt.thumbhash,
    officials: opt.officials
  })
  query.value = ''
  options.value = []
}
</script>

<template>
  <KunAutocomplete
    v-model="query"
    :options="options"
    :loading="isLoading"
    :debounce="300"
    manual-filter
    clearable
    :placeholder="placeholder ?? '输入游戏名搜索'"
    loading-text="搜索中…"
    no-result-text="无匹配结果"
    @search="onSearch"
    @select="onSelect"
  >
    <template #option="{ option }">
      <div class="flex items-center gap-3">
        <div class="bg-default-100 h-10 w-16 shrink-0 overflow-hidden rounded">
          <KunImage
            v-if="option.banner"
            :src="option.banner"
            :thumbhash="option.thumbhash"
            width="64"
            height="40"
            object-fit="cover"
            class-name="h-full w-full"
          />
        </div>
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">{{ option.label }}</p>
          <p
            v-if="option.officials?.length"
            class="text-default-500 truncate text-xs"
          >
            {{ option.officials.join('、') }}
          </p>
        </div>
      </div>
    </template>
  </KunAutocomplete>
</template>
