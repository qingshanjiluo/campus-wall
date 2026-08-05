<script setup lang="ts">
// Schema-driven edit form (infra doc 21 §2.7): renders every projected field
// by its kind + the host's presentation config, tracks a working copy, and
// emits ONLY the dirty subset as the proposal patch (field key → new value).
// Zero policy logic — capabilities come from the projection.
import { computed, reactive, ref, toRaw, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import type { EditFieldConfigMap, EditSchemaField } from './types'
import { editValueEqual } from './utils'

const props = withDefaults(
  defineProps<{
    fields: EditSchemaField[]
    /** Current entity values keyed by eternal field keys. */
    values: Record<string, unknown>
    config: EditFieldConfigMap
    /** Section order; fields whose config.group is absent land in the last
     * unnamed section. */
    groupOrder?: string[]
    disabled?: boolean
    /** 'stack' (default) renders every section top-to-bottom. 'tabs' renders
     * one section at a time behind a tab rail — vertical (left) on desktop,
     * horizontal (top) on mobile — with a per-tab "edited" marker. */
    layout?: 'stack' | 'tabs'
    /** Group names (in the `tabs` layout) whose fields render behind their OWN
     * horizontal sub-tab — one field at a time — instead of stacked. Handy for
     * a language switch (e.g. the four intro fields as 英语/日语/… tabs). Each
     * sub-tab uses the field's `tabLabel` (falling back to `label`). */
    tabbedGroups?: string[]
  }>(),
  { layout: 'stack' }
)

const emit = defineEmits<{
  'update:patch': [patch: Record<string, unknown>]
}>()

// Working copy, (re)seeded whenever the upstream values change identity.
const working = reactive<Record<string, unknown>>({})
watch(
  () => props.values,
  (values) => {
    for (const key of Object.keys(working)) {
      Reflect.deleteProperty(working, key)
    }
    for (const field of props.fields) {
      working[field.key] = structuredClone(toRaw(values[field.key]) ?? null)
    }
  },
  { immediate: true, deep: false }
)

const patch = computed<Record<string, unknown>>(() => {
  const out: Record<string, unknown> = {}
  for (const field of props.fields) {
    if (field.locked || field.deprecated || !field.can_propose) {
      continue
    }
    const baseline = props.values[field.key] ?? null
    const current = working[field.key] ?? null
    if (!editValueEqual(baseline, current)) {
      out[field.key] = current
    }
  }
  return out
})
watch(patch, (value) => emit('update:patch', value), { deep: false })

const dirtyCount = computed(() => Object.keys(patch.value).length)
defineExpose({ dirtyCount })

// Group fields into ordered sections.
const UNGROUPED = '__ungrouped'
const sections = computed(() => {
  const byGroup = new Map<string, EditSchemaField[]>()
  for (const field of props.fields) {
    if (field.deprecated) {
      continue
    }
    const group = props.config[field.key]?.group ?? ''
    const bucket = byGroup.get(group)
    if (bucket) {
      bucket.push(field)
    } else {
      byGroup.set(group, [field])
    }
  }
  const order = props.groupOrder ?? [...byGroup.keys()]
  const out: { name: string; fields: EditSchemaField[] }[] = []
  for (const name of order) {
    const fields = byGroup.get(name)
    if (fields?.length) {
      out.push({ name, fields })
      byGroup.delete(name)
    }
  }
  for (const [name, fields] of byGroup) {
    out.push({ name, fields })
  }
  return out
})

// ---- tabbed layout ---------------------------------------------------------
// Vertical rail on desktop, horizontal on mobile. Panels stay mounted (v-show)
// so entity-picker name lookups + dirty state survive tab switches.
const isDesktop = useMediaQuery('(min-width: 768px)')
const tabOrientation = computed(() => (isDesktop.value ? 'vertical' : 'horizontal'))

const tabKey = (name: string) => name || UNGROUPED

// patch-key → its section group, so a tab can flag "you edited this section".
const dirtyBySection = computed<Record<string, number>>(() => {
  const counts: Record<string, number> = {}
  for (const key of Object.keys(patch.value)) {
    const group = props.config[key]?.group ?? ''
    counts[tabKey(group)] = (counts[tabKey(group)] ?? 0) + 1
  }
  return counts
})

const tabItems = computed(() =>
  sections.value.map((section) => ({
    value: tabKey(section.name),
    textValue: section.name || '其他',
    // # of unsaved edits in this section → a count badge in the #tab slot
    // (KunTab 2.15 typed the extra field through the generic item shape).
    count: dirtyBySection.value[tabKey(section.name)] ?? 0
  }))
)

const active = ref('')
watch(
  sections,
  (list) => {
    if (!list.some((s) => tabKey(s.name) === active.value)) {
      active.value = list.length ? tabKey(list[0]!.name) : ''
    }
  },
  { immediate: true }
)

// ---- tabbed groups (a group's fields behind their own sub-tab) -------------
const isTabbedGroup = (name: string) =>
  props.tabbedGroups?.includes(name) ?? false

// section name → the field key shown in its sub-tab.
const activeField = reactive<Record<string, string>>({})
watch(
  sections,
  (list) => {
    for (const section of list) {
      if (!isTabbedGroup(section.name)) {
        continue
      }
      const keys = section.fields.map((f) => f.key)
      if (!keys.includes(activeField[section.name] ?? '')) {
        activeField[section.name] = keys[0] ?? ''
      }
    }
  },
  { immediate: true }
)

const subTabItems = (section: { name: string; fields: EditSchemaField[] }) =>
  section.fields.map((field) => ({
    value: field.key,
    textValue:
      props.config[field.key]?.tabLabel ??
      props.config[field.key]?.label ??
      field.key,
    // a dot marks a language/field with unsaved edits.
    dirty: field.key in patch.value
  }))
</script>

<template>
  <!-- Tabbed layout: rail + one visible panel. -->
  <div v-if="layout === 'tabs'" class="flex flex-col gap-4 md:flex-row md:gap-6">
    <KunTab
      :model-value="active"
      :items="tabItems"
      :orientation="tabOrientation"
      variant="pills"
      color="primary"
      size="md"
      class="md:w-44 md:shrink-0"
      @update:model-value="(value) => (active = value)"
    >
      <template #tab="{ item }">
        {{ item.textValue }}
        <KunBadge
          v-if="item.count"
          variant="count"
          :count="item.count"
          color="danger"
          size="sm"
        />
      </template>
    </KunTab>
    <div class="min-w-0 flex-1">
      <section
        v-for="section in sections"
        v-show="tabKey(section.name) === active"
        :key="section.name"
        class="grid grid-cols-1 gap-5"
      >
        <!-- Tabbed group: fields behind a horizontal sub-tab (e.g. intro
             languages), one at a time. -->
        <template v-if="isTabbedGroup(section.name)">
          <KunTab
            :model-value="activeField[section.name] ?? ''"
            :items="subTabItems(section)"
            orientation="horizontal"
            variant="underlined"
            color="primary"
            size="sm"
            @update:model-value="(value) => (activeField[section.name] = value)"
          >
            <template #tab="{ item }">
              {{ item.textValue }}
              <KunBadge
                v-if="item.dirty"
                variant="dot"
                color="danger"
                size="sm"
              />
            </template>
          </KunTab>
          <EditkitSchemaField
            v-for="field in section.fields"
            v-show="field.key === activeField[section.name]"
            :key="field.key"
            v-model="working[field.key]"
            :field="field"
            :config="config[field.key]"
            :disabled="disabled"
          />
        </template>
        <!-- Normal: fields stacked. -->
        <template v-else>
          <EditkitSchemaField
            v-for="field in section.fields"
            :key="field.key"
            v-model="working[field.key]"
            :field="field"
            :config="config[field.key]"
            :disabled="disabled"
          />
        </template>
      </section>
    </div>
  </div>

  <!-- Stacked layout (default): every section top-to-bottom. -->
  <div v-else class="space-y-6">
    <section v-for="section in sections" :key="section.name" class="space-y-3">
      <h3
        v-if="section.name"
        class="text-default-900 border-b pb-1 text-base font-semibold"
      >
        {{ section.name }}
      </h3>
      <div class="grid grid-cols-1 gap-4">
        <EditkitSchemaField
          v-for="field in section.fields"
          :key="field.key"
          v-model="working[field.key]"
          :field="field"
          :config="config[field.key]"
          :disabled="disabled"
        />
      </div>
    </section>
  </div>
</template>
