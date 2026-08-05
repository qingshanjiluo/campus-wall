<script setup lang="ts">
// The append-only revision log, newest-first: action badges (with honest
// migrated-history provenance — the legacy_action badge), precise
// changed-field chips, double attribution, and a pick-any-two diff selector.
import { computed, ref, watch } from 'vue'
import type { EditRevision, EditUser } from './types'
import { revisionActionBadge } from './utils'

const props = defineProps<{
  items: EditRevision[]
  users?: Record<number, EditUser>
  labelFor: (key: string) => string
  /** Extra/override labels for legacy action words (host vocabulary). */
  legacyActionLabels?: Record<string, string>
}>()

const emit = defineEmits<{
  diff: [fromSeq: number, toSeq: number]
}>()

const selected = ref<number[]>([])
watch(
  () => props.items,
  (items) => {
    // Default to the two newest versions (items are newest-first) so a diff is
    // one click away.
    selected.value = items.slice(0, 2).map((r) => r.seq)
  },
  { immediate: true }
)

// Changed fields can be many — a wall of gray chips reads poorly. Show a muted,
// capped label summary instead.
const changedSummary = (keys: string[]) => {
  const labels = keys.map((k) => props.labelFor(k))
  const CAP = 8
  return labels.length <= CAP
    ? labels.join('、')
    : `${labels.slice(0, CAP).join('、')} … 共 ${labels.length} 项`
}

const toggle = (seq: number) => {
  const index = selected.value.indexOf(seq)
  if (index >= 0) {
    selected.value.splice(index, 1)
    return
  }
  // Keep at most two picks: the oldest pick rolls off.
  if (selected.value.length === 2) {
    selected.value.shift()
  }
  selected.value.push(seq)
}

const canDiff = computed(() => selected.value.length === 2)
const requestDiff = () => {
  if (!canDiff.value) {
    return
  }
  const [a, b] = [...selected.value].sort((x, y) => x - y)
  emit('diff', a!, b!)
}

const legacyLabel = (word: string) =>
  props.legacyActionLabels?.[word] ?? word
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <p class="text-default-500 text-sm">点击任意两个版本进行对比</p>
      <KunButton
        size="sm"
        color="primary"
        variant="flat"
        :disabled="!canDiff"
        @click="requestDiff"
      >
        对比所选版本
      </KunButton>
    </div>

    <KunNull v-if="!items.length" description="暂无修订记录" />

    <div v-else class="space-y-2">
      <div
        v-for="revision in items"
        :key="revision.id"
        class="relative cursor-pointer rounded border p-3 transition"
        :class="
          selected.includes(revision.seq)
            ? 'border-primary bg-primary/5 ring-primary ring-1'
            : 'border-default-200 hover:border-default-300'
        "
        @click="toggle(revision.seq)"
      >
        <div class="flex items-start gap-3">
          <!-- Selection indicator, top-right corner: an empty box hints the row
               is pickable, a primary check marks the pick (color="primary"). -->
          <KunCheckBox
            :model-value="selected.includes(revision.seq)"
            color="primary"
            size="sm"
            class="pointer-events-none absolute right-3 top-3 shrink-0"
          />
          <div class="min-w-0 flex-1 space-y-2 pr-7">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-default-700 text-sm font-semibold">
                #{{ revision.seq }}
              </span>
              <KunChip
                size="sm"
                variant="flat"
                :color="revisionActionBadge(revision.action).color"
              >
                {{ revisionActionBadge(revision.action).label }}
              </KunChip>
              <KunChip
                v-if="revision.legacy_action"
                size="sm"
                variant="flat"
                color="warning"
              >
                迁移 · {{ legacyLabel(revision.legacy_action) }}
              </KunChip>
              <KunChip
                v-if="revision.legacy_minor"
                size="sm"
                variant="flat"
                color="default"
              >
                小修改
              </KunChip>
              <span class="text-default-400 ml-auto text-xs">
                <KunTime :time="revision.created_at" type="date" show-year />
              </span>
            </div>

            <p
              v-if="revision.changed_fields?.length"
              class="text-default-500 text-xs"
            >
              <span class="text-default-400">修改字段：</span>
              {{ changedSummary(revision.changed_fields) }}
            </p>

            <p v-if="revision.legacy_note" class="text-default-500 text-sm">
              {{ revision.legacy_note }}
            </p>

            <!-- Attribution: the editor (actor) and, when a reviewer amended
                 before merge, the reviewer — each an avatar + name chip. -->
            <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
              <span class="text-default-500 flex items-center gap-1.5 text-xs">
                <span class="text-default-400">编辑者</span>
                <KunUserChip
                  v-if="users?.[revision.actor_uid]"
                  :user="users?.[revision.actor_uid]"
                  size="sm"
                  :is-navigation="false"
                  :disable-floating="true"
                />
                <span v-else>用户 #{{ revision.actor_uid }}</span>
              </span>
              <span
                v-if="revision.amender_uid"
                class="text-default-500 flex items-center gap-1.5 text-xs"
              >
                <span class="text-default-400">审核者</span>
                <KunUserChip
                  v-if="users?.[revision.amender_uid]"
                  :user="users?.[revision.amender_uid]"
                  size="sm"
                  :is-navigation="false"
                  :disable-floating="true"
                />
                <span v-else>用户 #{{ revision.amender_uid }}</span>
              </span>
            </div>

            <!-- Host-supplied per-revision actions (e.g. revert), bottom-right.
                 @click.stop so acting doesn't toggle this revision's pick. -->
            <div class="flex justify-end" @click.stop>
              <slot name="actions" :revision="revision" />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
