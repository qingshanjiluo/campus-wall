<script setup lang="ts">
// One schema-driven field control (used by EditkitSchemaForm, also standalone
// in the review workbench's amend flow). Renders by the resolved control,
// reflects the projection's capabilities (locked / no-propose → readonly with
// a reason chip), and emits TYPED values upward. Extraction-ready boundary:
// KunUI primitives + self-contained types only.
import { computed, ref, watch } from 'vue'
import { useSortable } from '@vueuse/integrations/useSortable'
import type {
  EditFieldConfig,
  EditObjectColumn,
  EditSchemaField
} from './types'
import {
  editValueEqual,
  formatEditItem,
  formatEditValue,
  resolveControl
} from './utils'

const props = defineProps<{
  field: EditSchemaField
  config?: EditFieldConfig
  modelValue: unknown
  /** Force readonly regardless of capabilities (e.g. while submitting). */
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const control = computed(() => resolveControl(props.field, props.config))
const label = computed(() => props.config?.label ?? props.field.key)

// Image kinds edit through the host's upload hook (E3b) — without one they
// stay display-only; deprecated fields render but never edit.
const readonlyReason = computed(() => {
  if (props.field.locked) {
    return '锁定字段'
  }
  if (props.field.deprecated) {
    return '已废弃'
  }
  if (!props.field.can_propose) {
    return '无编辑权限'
  }
  if (
    (control.value === 'image' || control.value === 'image-list') &&
    !props.config?.uploadImage
  ) {
    return '本期只读'
  }
  if (control.value === 'readonly') {
    return '本期只读'
  }
  return ''
})
const editable = computed(() => !props.disabled && readonlyReason.value === '')

// ---- typed buffers ---------------------------------------------------------
// String-ish controls edit a text buffer and convert on emit; list controls
// edit structured local copies. Buffers re-sync when the upstream value
// changes identity (e.g. the form resets).

const textBuffer = ref('')
const boolBuffer = ref(false)
const stringList = ref<string[]>([])
type ObjectRow = Record<string, unknown>
const objectRows = ref<ObjectRow[]>([])

const syncFromValue = () => {
  const v = props.modelValue
  switch (control.value) {
    case 'switch':
      boolBuffer.value = v === true
      break
    case 'string-list':
      stringList.value = Array.isArray(v) ? v.map((x) => String(x)) : []
      break
    case 'number-list':
      stringList.value = Array.isArray(v) ? v.map((x) => String(x)) : []
      break
    case 'object-list':
      objectRows.value = Array.isArray(v)
        ? (v as ObjectRow[]).map((row) => ({ ...row }))
        : []
      break
    default:
      textBuffer.value = v === null || v === undefined ? '' : String(v)
  }
}
watch(() => props.modelValue, syncFromValue, { immediate: true })

const emitText = (raw: string | number) => {
  const value = String(raw)
  textBuffer.value = value
  switch (control.value) {
    case 'number': {
      const trimmed = value.trim()
      if (trimmed === '') {
        emit('update:modelValue', props.config?.nullable ? null : 0)
        return
      }
      const n = Number(trimmed)
      emit('update:modelValue', Number.isFinite(n) ? n : trimmed)
      return
    }
    case 'date':
      emit('update:modelValue', value === '' ? null : value)
      return
    default:
      emit('update:modelValue', value)
  }
}

const onDatePicked = (
  value: string | null | [string | null, string | null]
) => {
  const single = Array.isArray(value) ? (value[0] ?? null) : value
  emit('update:modelValue', single || null)
}

const emitSelect = (value: string | number | (string | number)[] | null) => {
  // Single-select only in this form; unwrap a defensive array shape.
  emit('update:modelValue', Array.isArray(value) ? (value[0] ?? null) : value)
}

const emitSwitch = (value: boolean) => {
  boolBuffer.value = value
  emit('update:modelValue', value)
}

const emitStringList = (items: string[]) => {
  stringList.value = items
  if (control.value === 'number-list') {
    emit(
      'update:modelValue',
      items
        .map((x) => Number(x.trim()))
        .filter((n) => Number.isInteger(n) && n > 0)
    )
    return
  }
  emit(
    'update:modelValue',
    items.map((x) => x.trim()).filter((x) => x.length > 0)
  )
}

const objectColumns = computed(() => props.config?.columns ?? [])

// A select column edits a string, but the underlying key may be numeric (a
// title's `kind` is an enum int). Coerce back to the option's own type so the
// engine sees the value it declared rather than its decimal spelling.
const coerceColumnValue = (col: EditObjectColumn, raw: string): unknown => {
  const match = (col.options ?? []).find((o) => String(o.value) === raw)
  return match ? match.value : raw
}

// A row is dropped only when EVERY declared column is blank — a title with a
// language and no text is a mistake the user is mid-way through, not something
// to silently discard while they type. The rest of the row is spread through
// untouched: the engine rejects a value that lost a key it wrote.
const emitObjectRows = () => {
  const cols = objectColumns.value
  emit(
    'update:modelValue',
    objectRows.value
      .filter((row) =>
        cols.some((c) => String(row[c.key] ?? '').trim() !== '')
      )
      .map((row) => {
        const out: ObjectRow = { ...row }
        for (const c of cols) {
          if (typeof out[c.key] === 'string') {
            out[c.key] = (out[c.key] as string).trim()
          }
        }
        return out
      })
  )
}

const addObjectRow = () => {
  const blank: ObjectRow = {}
  for (const c of objectColumns.value) {
    blank[c.key] = c.control === 'select' ? (c.options?.[0]?.value ?? '') : ''
  }
  objectRows.value.push({ ...blank, ...(props.config?.newRow?.() ?? {}) })
}

const removeObjectRow = (index: number) => {
  objectRows.value.splice(index, 1)
  emitObjectRows()
}

const selectOptions = computed(() =>
  (props.config?.options ?? []).map((o) => ({ value: o.value, label: o.label }))
)

// Image rendering: single hash/URL or an item list.
const resolveImageURL = (v: unknown) =>
  props.config?.resolveImage ? props.config.resolveImage(v) : ''

const imageURLs = computed(() => {
  if (control.value === 'image') {
    const url = resolveImageURL(props.modelValue)
    return url ? [url] : []
  }
  if (control.value === 'image-list' && Array.isArray(props.modelValue)) {
    return props.modelValue.map(resolveImageURL).filter((u) => u !== '')
  }
  return []
})

// ---- image editing (E3b: upload + add/remove/reorder through the host's
// upload hook; the values stay opaque to the kit) -------------------------

const imageItems = computed<unknown[]>(() =>
  Array.isArray(props.modelValue) ? (props.modelValue as unknown[]) : []
)

const isUploading = ref(false)
const uploadProgress = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const pickFiles = () => fileInput.value?.click()

const onFilesPicked = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? []).filter((f) =>
    f.type.startsWith('image/')
  )
  input.value = ''
  const upload = props.config?.uploadImage
  if (!upload || files.length === 0 || isUploading.value) {
    return
  }
  isUploading.value = true
  try {
    if (control.value === 'image') {
      const file = files[0]
      if (!file) {
        return
      }
      uploadProgress.value = '上传中'
      const value = await upload(file, [])
      if (value !== null && value !== undefined) {
        emit('update:modelValue', value)
      }
      return
    }
    // image-list: sequential uploads so the progress counter is honest.
    let items = [...imageItems.value]
    for (const [i, file] of files.entries()) {
      uploadProgress.value =
        files.length > 1 ? `上传中 ${i + 1}/${files.length}` : '上传中'
      const item = await upload(file, items)
      if (item !== null && item !== undefined) {
        items = [...items, item]
      }
    }
    emitImageItems(items)
  } finally {
    isUploading.value = false
    uploadProgress.value = ''
  }
}

const emitImageItems = (items: unknown[]) => {
  const normalize = props.config?.normalizeItems
  emit('update:modelValue', normalize ? normalize(items) : items)
}

const clearImage = () => emit('update:modelValue', '')

const removeImageItem = (index: number) => {
  emitImageItems(imageItems.value.filter((_, i) => i !== index))
}

// Drag-to-reorder for the image list (sortablejs via @vueuse). useSortable
// mutates the local `sortItems` mirror on drop; we push the new order up
// through emitImageItems (which re-stamps sort_order). The editValueEqual
// guards keep the mirror ↔ modelValue sync from looping (normalizeItems
// restamps sort_order, so a reorder does change the emitted items).
const gridRef = ref<HTMLElement | null>(null)
const sortItems = ref<unknown[]>([...imageItems.value])
watch(imageItems, (items) => {
  if (!editValueEqual(items, sortItems.value)) {
    sortItems.value = [...items]
  }
})
watch(sortItems, (items) => {
  if (!editValueEqual(items, imageItems.value)) {
    emitImageItems([...items])
  }
})
useSortable(gridRef, sortItems, {
  animation: 150,
  handle: '.ek-drag-handle',
  draggable: '.ek-image-item'
})

const pinImageItem = (index: number) => {
  const items = [...imageItems.value]
  const picked = items.splice(index, 1)
  emitImageItems([...picked, ...items])
}
</script>

<template>
  <div class="space-y-1">
    <div class="flex items-center gap-2">
      <span class="text-default-700 text-sm font-medium">{{ label }}</span>
      <KunChip v-if="readonlyReason" size="sm" variant="flat" color="default">
        {{ readonlyReason }}
      </KunChip>
    </div>

    <!-- Host escape hatch: a custom control component (e.g. a rich editor). -->
    <template v-if="config?.component">
      <component
        :is="config.component"
        :model-value="modelValue"
        :disabled="!editable"
        @update:model-value="(value: unknown) => emit('update:modelValue', value)"
      />
      <p v-if="config?.description" class="text-default-400 text-xs">
        {{ config.description }}
      </p>
    </template>

    <!-- Entity+kind picker: one row per (entity, kind) edge, so the same
         entity can be attached twice under different kinds. -->
    <template
      v-else-if="
        control === 'entity-kind-picker' &&
        config?.searchEntities &&
        config?.entityIdKey &&
        config?.entityKinds
      "
    >
      <EditkitEntityKindPicker
        :model-value="modelValue"
        :disabled="!editable"
        :placeholder="config?.placeholder"
        :id-key="config.entityIdKey"
        :kind-options="config.entityKinds"
        :default-kind="config?.entityDefaultKind ?? config.entityKinds[0]!.value"
        :search="config.searchEntities"
        :resolve="config?.resolveEntities"
        @update:model-value="(value) => emit('update:modelValue', value)"
      />
      <p v-if="config?.description" class="text-default-400 text-xs">
        {{ config.description }}
      </p>
    </template>

    <!-- Entity picker: search + pick by NAME, store id(s). Renders read-only
         too (names still resolve) via :disabled. -->
    <template v-else-if="control === 'entity-picker' && config?.searchEntities">
      <EditkitEntityPicker
        :model-value="modelValue"
        :multiple="config?.multiple"
        :disabled="!editable"
        :placeholder="config?.placeholder"
        :search="config.searchEntities"
        :resolve="config?.resolveEntities"
        @update:model-value="(value) => emit('update:modelValue', value)"
      />
      <p v-if="config?.description" class="text-default-400 text-xs">
        {{ config.description }}
      </p>
    </template>

    <!-- Readonly rendering: images as previews, everything else as text -->
    <template v-else-if="!editable">
      <div
        v-if="imageURLs.length"
        class="flex flex-wrap items-start gap-2"
      >
        <img
          v-for="(url, i) in imageURLs"
          :key="i"
          :src="url"
          loading="lazy"
          class="max-h-24 max-w-full rounded object-cover"
        />
      </div>
      <p v-else class="text-default-500 text-sm break-all whitespace-pre-wrap">
        {{ formatEditValue(modelValue, config) }}
      </p>
    </template>

    <template v-else>
      <KunTextarea
        v-if="control === 'textarea'"
        :model-value="textBuffer"
        :placeholder="config?.placeholder"
        :description="config?.description"
        @update:model-value="emitText"
      />
      <KunSelect
        v-else-if="control === 'select'"
        :model-value="(modelValue as string | number | null)"
        :options="selectOptions"
        :description="config?.description"
        @update:model-value="emitSelect"
      />
      <KunSwitch
        v-else-if="control === 'switch'"
        :model-value="boolBuffer"
        :label="config?.description ?? ''"
        @update:model-value="emitSwitch"
      />
      <KunTagInput
        v-else-if="control === 'string-list' || control === 'number-list'"
        :model-value="stringList"
        :placeholder="config?.placeholder ?? '输入后回车添加'"
        :description="config?.description"
        @update:model-value="emitStringList"
      />
      <div v-else-if="control === 'object-list'" class="space-y-2">
        <div
          v-for="(row, index) in objectRows"
          :key="index"
          class="flex items-start gap-2"
        >
          <template v-for="col in objectColumns" :key="col.key">
            <KunSelect
              v-if="col.control === 'select'"
              :model-value="String(row[col.key] ?? '')"
              :options="(col.options ?? []).map((o) => ({ value: String(o.value), label: o.label }))"
              :class-name="col.width ?? 'flex-1'"
              @update:model-value="
                (v: string | string[] | null) => {
                  row[col.key] = coerceColumnValue(col, String(v ?? ''))
                  emitObjectRows()
                }
              "
            />
            <KunTextarea
              v-else-if="col.control === 'textarea'"
              v-model="row[col.key] as string"
              :placeholder="col.placeholder ?? col.label"
              :class-name="col.width ?? 'flex-1'"
              @update:model-value="emitObjectRows"
            />
            <KunInput
              v-else
              v-model="row[col.key] as string"
              :placeholder="col.placeholder ?? col.label"
              :class-name="col.width ?? 'flex-1'"
              @update:model-value="emitObjectRows"
            />
          </template>
          <KunButton
            :is-icon-only="true"
            variant="light"
            color="danger"
            size="sm"
            @click="removeObjectRow(index)"
          >
            <KunIcon name="lucide:x" />
          </KunButton>
        </div>
        <KunButton
          variant="flat"
          color="default"
          size="sm"
          @click="addObjectRow"
        >
          <KunIcon name="lucide:plus" />
          添加一行
        </KunButton>
        <p v-if="config?.description" class="text-default-400 text-xs">
          {{ config.description }}
        </p>
      </div>
      <!-- Single image (E3b): preview + replace/clear through the host's
           upload hook. The stored value stays opaque to the kit. -->
      <div v-else-if="control === 'image'" class="space-y-2">
        <img
          v-if="imageURLs[0]"
          :src="imageURLs[0]"
          loading="lazy"
          class="max-h-32 max-w-full rounded object-cover"
        />
        <div class="flex items-center gap-2">
          <KunButton
            variant="flat"
            color="default"
            size="sm"
            :disabled="isUploading"
            @click="pickFiles"
          >
            <KunIcon name="lucide:upload" />
            {{ isUploading ? uploadProgress : imageURLs[0] ? '更换图片' : '上传图片' }}
          </KunButton>
          <KunButton
            v-if="imageURLs[0]"
            variant="light"
            color="danger"
            size="sm"
            :disabled="isUploading"
            @click="clearImage"
          >
            移除
          </KunButton>
        </div>
        <p v-if="config?.description" class="text-default-400 text-xs">
          {{ config.description }}
        </p>
      </div>

      <!-- Image list (E3b): upload/append, remove, reorder; item 0 renders
           the host's pinned badge (e.g. the cover set's banner). -->
      <div v-else-if="control === 'image-list'" class="space-y-2">
        <div
          ref="gridRef"
          class="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4"
        >
          <div
            v-for="(item, index) in sortItems"
            :key="resolveImageURL(item) || index"
            class="ek-image-item border-default-200 group relative overflow-hidden rounded border"
            :class="{
              'ring-primary border-primary ring-2':
                config?.pinFirstLabel && index === 0
            }"
          >
            <img
              :src="resolveImageURL(item)"
              loading="lazy"
              class="aspect-video w-full object-cover"
            />
            <!-- Drag handle: grip to reorder (sortablejs). -->
            <div
              class="ek-drag-handle absolute top-1 left-1 flex cursor-move items-center rounded bg-black/50 p-1 text-white"
              title="拖动排序"
            >
              <KunIcon name="lucide:grip-vertical" class="h-4 w-4" />
            </div>
            <KunChip
              v-if="config?.pinFirstLabel && index === 0"
              color="primary"
              variant="solid"
              size="sm"
              class="pointer-events-none absolute bottom-1 left-1"
            >
              {{ config.pinFirstLabel }}
            </KunChip>
            <div class="absolute top-1 right-1 flex gap-1">
              <KunButton
                v-if="config?.pinFirstLabel && index > 0"
                :is-icon-only="true"
                size="sm"
                variant="solid"
                color="default"
                title="设为封面"
                @click="pinImageItem(index)"
              >
                <KunIcon name="lucide:pin" />
              </KunButton>
              <KunButton
                :is-icon-only="true"
                size="sm"
                variant="solid"
                color="danger"
                title="移除"
                @click="removeImageItem(index)"
              >
                <KunIcon name="lucide:trash-2" />
              </KunButton>
            </div>
          </div>
        </div>
        <!-- Add button lives OUTSIDE the sortable grid so it never offsets the
             drag indices. -->
        <button
          type="button"
          class="border-default-200 text-default-400 hover:border-primary hover:text-primary flex w-full cursor-pointer items-center justify-center gap-1 rounded border border-dashed py-3 text-sm"
          :disabled="isUploading"
          @click="pickFiles"
        >
          <KunIcon :name="isUploading ? 'lucide:loader' : 'lucide:plus'" />
          {{ isUploading ? uploadProgress : '添加图片' }}
        </button>
        <p
          v-if="sortItems.length > 1"
          class="text-default-400 flex items-center gap-1 text-xs"
        >
          <KunIcon name="lucide:grip-vertical" class="h-3 w-3 shrink-0" />
          拖动图片左上角的按钮可调整顺序
        </p>
        <p v-if="config?.description" class="text-default-400 text-xs">
          {{ config.description }}
        </p>
      </div>

      <template v-else-if="control === 'date'">
        <KunDatePicker
          :model-value="(modelValue as string | null) ?? null"
          mode="single"
          :placeholder="config?.placeholder"
          @update:model-value="onDatePicked"
        />
        <p v-if="config?.description" class="text-default-400 text-xs">
          {{ config.description }}
        </p>
      </template>

      <KunInput
        v-else
        :model-value="textBuffer"
        :type="control === 'number' ? 'number' : 'text'"
        :placeholder="config?.placeholder"
        :description="config?.description"
        @update:model-value="emitText"
      />

      <input
        v-if="control === 'image' || control === 'image-list'"
        ref="fileInput"
        type="file"
        accept="image/*"
        :multiple="control === 'image-list'"
        class="hidden"
        @change="onFilesPicked"
      />
    </template>
  </div>
</template>
