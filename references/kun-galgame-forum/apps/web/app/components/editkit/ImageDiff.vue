<script setup lang="ts">
// Unified image diff: one strip of thumbnails, each tagged − or +, replacing
// the 修改前 / 修改后 columns this started as.
//
// Those columns had the same flaw as the text ones — and one worse bug. Each
// side fell back to printing the WHOLE field as text when its own side had no
// changes, so adding a screenshot rendered the entire old gallery as a wall of
// URLs on the left and the one new picture on the right. The reader is judging
// "which pictures changed"; every unchanged URL in that list is noise, and a
// URL is not a picture.
//
// Unchanged images are counted, not drawn. A gallery of 20 that gained 1 should
// show one thumbnail, not 21.

import { computed } from 'vue'
import type { ImageDiffEntry } from './types'

const props = defineProps<{
  removed: ImageDiffEntry[]
  added: ImageDiffEntry[]
  keptCount: number
}>()

const hasChange = computed(
  () => props.removed.length > 0 || props.added.length > 0
)
</script>

<template>
  <div class="space-y-1">
    <div v-if="hasChange" class="flex flex-wrap items-center gap-2">
      <span
        v-if="added.length"
        class="text-success-600 text-[10px] tabular-nums"
      >
        +{{ added.length }}
      </span>
      <span
        v-if="removed.length"
        class="text-danger-600 text-[10px] tabular-nums"
      >
        −{{ removed.length }}
      </span>
      <span v-if="keptCount" class="text-default-400 text-[10px] tabular-nums">
        {{ keptCount }} 张未改动
      </span>
    </div>

    <div
      class="border-default-200 bg-content1 flex flex-wrap gap-2 rounded-lg border px-2 py-2"
    >
      <!-- Removed first, then added: the same reading order as a text diff. -->
      <figure
        v-for="(entry, i) in [
          ...removed.map((e) => ({ ...e, op: 'delete' as const })),
          ...added.map((e) => ({ ...e, op: 'insert' as const }))
        ]"
        :key="`${entry.op}-${i}`"
        class="relative"
      >
        <img
          v-if="entry.url"
          :src="entry.url"
          loading="lazy"
          class="max-h-24 rounded border-2 object-cover"
          :class="
            entry.op === 'delete'
              ? 'border-danger/60 opacity-70 grayscale'
              : 'border-success/60'
          "
        />
        <!-- No preview resolvable — name the item, never dump the field. -->
        <KunChip
          v-else
          size="sm"
          variant="flat"
          :color="entry.op === 'delete' ? 'danger' : 'success'"
        >
          {{ entry.op === 'delete' ? '−' : '+' }} {{ entry.text }}
        </KunChip>
        <figcaption
          v-if="entry.url"
          class="absolute top-1 left-1 rounded px-1 text-[10px] font-semibold text-white"
          :class="entry.op === 'delete' ? 'bg-danger' : 'bg-success'"
        >
          {{ entry.op === 'delete' ? '−' : '+' }}
        </figcaption>
      </figure>

      <p v-if="!hasChange" class="text-default-400 text-xs">
        <template v-if="keptCount"
          >仅顺序调整（{{ keptCount }} 张未改动）</template
        >
        <template v-else>（空）</template>
      </p>
    </div>
  </div>
</template>
