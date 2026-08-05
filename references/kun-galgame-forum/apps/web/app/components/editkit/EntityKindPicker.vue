<script setup lang="ts">
// Async entity picker whose entries carry a KIND (editkit).
//
// EntityPicker stores a bare id array, which cannot express the one thing this
// field exists for: the SAME entity attached twice under different kinds — a
// 会社 that both developed and published a game. The registry models one edge
// per (entity, kind), so the stored value is one object per edge and the
// identity of a row is the PAIR, never the id alone.
//
// Host-agnostic like its sibling: search + id→name resolve arrive as props, and
// the id key is a prop too, so the value is the field's own wire shape
// ([{label_id, kind}]) with no translation layer between control and patch —
// a translation layer is exactly where a shape disagreement hides.
import { computed, ref, watch } from 'vue'
import type { EditSelectOption } from './types'

interface KindOption {
  value: number
  label: string
}

const props = defineProps<{
  modelValue: unknown
  disabled?: boolean
  placeholder?: string
  /** Object key holding the entity id (e.g. "label_id"). */
  idKey: string
  /** The kind vocabulary, in the order it should be offered. */
  kindOptions: KindOption[]
  /** Kind applied to a freshly picked entity. */
  defaultKind: number
  /** keyword → {value:id, label:name} options. */
  search: (keyword: string) => Promise<EditSelectOption[]>
  /** current ids → {value:id, label:name} (seeds names for existing picks). */
  resolve?: (
    ids: (string | number)[]
  ) => Promise<EditSelectOption[]> | EditSelectOption[]
}>()

const emit = defineEmits<{ 'update:modelValue': [value: unknown] }>()

interface Edge {
  id: number
  kind: number
}

const labels = ref(new Map<number, string>())
const labelFor = (id: number) => labels.value.get(id) ?? `#${id}`
const kindLabel = (kind: number) =>
  props.kindOptions.find((k) => k.value === kind)?.label ?? String(kind)

// The stored value → edges. Rows that carry no usable id are dropped rather
// than rendered as "#NaN": they cannot round-trip anyway.
const edges = computed<Edge[]>(() => {
  const v = props.modelValue
  if (!Array.isArray(v)) {
    return []
  }
  return (v as Record<string, unknown>[]).flatMap((row) => {
    const id = Number(row?.[props.idKey])
    if (!Number.isFinite(id) || id <= 0) {
      return []
    }
    return [{ id, kind: Number(row?.kind ?? props.defaultKind) }]
  })
})

const commit = (next: Edge[]) => {
  emit(
    'update:modelValue',
    next.map((e) => ({ [props.idKey]: e.id, kind: e.kind }))
  )
}

// Resolve names for any ids we don't know yet. One entity attached twice needs
// its name looked up once, so the ids are deduped before asking.
watch(
  edges,
  async (rows) => {
    const missing = [...new Set(rows.map((r) => r.id))].filter(
      (id) => !labels.value.has(id)
    )
    if (!missing.length || !props.resolve) {
      return
    }
    const resolved = await props.resolve(missing)
    const next = new Map(labels.value)
    for (const option of resolved) {
      next.set(Number(option.value), option.label)
    }
    labels.value = next
  },
  { immediate: true }
)

// --- search ---
const query = ref('')
const options = ref<{ value: string; label: string }[]>([])
const isLoading = ref(false)
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
  const hits = await props.search(kw)
  if (seq !== searchSeq) {
    return // superseded by a newer search — drop stale hits
  }
  // Entities are NOT filtered out once picked: attaching the same 会社 a second
  // time under another kind is the point of this control. The duplicate guard
  // is on the (id, kind) pair, at add time.
  const next = new Map(labels.value)
  options.value = hits.map((o) => {
    next.set(Number(o.value), o.label)
    return { value: String(o.value), label: o.label }
  })
  labels.value = next
  isLoading.value = false
}

const onSelect = (opt: { value: string; label: string }) => {
  const id = Number(opt.value)
  const already = edges.value.some(
    (e) => e.id === id && e.kind === props.defaultKind
  )
  if (!already) {
    commit([...edges.value, { id, kind: props.defaultKind }])
  }
  query.value = ''
  options.value = []
}

const setKind = (index: number, kind: number) => {
  const next = edges.value.map((e, i) => (i === index ? { ...e, kind } : e))
  // Retyping a row onto a kind the same entity already holds would create a
  // duplicate edge, which the engine rejects. Collapse instead of erroring.
  const seen = new Set<string>()
  commit(
    next.filter((e) => {
      const key = `${e.id}:${e.kind}`
      if (seen.has(key)) {
        return false
      }
      seen.add(key)
      return true
    })
  )
}

const removeAt = (index: number) => {
  commit(edges.value.filter((_, i) => i !== index))
}
</script>

<template>
  <div class="space-y-2">
    <div v-if="edges.length" class="space-y-1.5">
      <div
        v-for="(edge, index) in edges"
        :key="`${edge.id}:${edge.kind}`"
        class="flex flex-wrap items-center gap-2"
      >
        <KunChip size="sm" variant="flat" color="primary">
          {{ labelFor(edge.id) }}
        </KunChip>
        <KunSelect
          v-if="!disabled"
          :model-value="edge.kind"
          :options="kindOptions"
          size="sm"
          class-name="w-32"
          @update:model-value="(value) => setKind(index, Number(value))"
        />
        <span v-else class="text-default-500 text-sm">
          {{ kindLabel(edge.kind) }}
        </span>
        <KunButton
          v-if="!disabled"
          variant="light"
          color="danger"
          size="sm"
          @click="removeAt(index)"
        >
          移除
        </KunButton>
      </div>
    </div>
    <KunAutocomplete
      v-if="!disabled"
      v-model="query"
      :options="options"
      :loading="isLoading"
      :debounce="300"
      manual-filter
      clearable
      :placeholder="placeholder ?? '输入名称搜索'"
      loading-text="搜索中…"
      no-result-text="无匹配结果"
      @search="onSearch"
      @select="onSelect"
    />
  </div>
</template>
