<script setup lang="ts">
import {
  KUN_GALGAME_OFFICIAL_CATEGORY_MAP,
  KUN_GALGAME_OFFICIAL_LANGUAGE_MAP
} from '~/constants/galgameOfficial'

// Renders a browse row OR a search hit. A hit carries identity only (the
// catalog's entity search is a picker feed), so everything below the name is
// optional here and simply absent rather than zero-filled — the card used to
// print "+ 0" and a blank category for every search result.
const props = defineProps<{
  official: GalgameOfficialItem | GalgameTaxonomySearchItem
}>()

const detail = computed(() =>
  'category' in props.official ? props.official : undefined
)

// The category vocabulary is the CATALOG's (game_brand / publisher /
// doujin_circle / …). The old three-case switch only knew the retired wiki
// words, so every catalog kind fell through to its raw English key — the browse
// list has been showing "publisher" and "group" chips.
const categoryText = (category: string) =>
  KUN_GALGAME_OFFICIAL_CATEGORY_MAP[category] || category
</script>

<template>
  <KunCard
    :is-transparent="false"
    :is-hoverable="true"
    :href="`/galgame/official/${official.id}`"
  >
    <h3 class="text-default-900 font-semibold">
      {{ official.name }}
      <KunChip v-if="detail" size="xs">
        {{ `+ ${detail.galgame_count}` }}
      </KunChip>
    </h3>
    <div v-if="detail" class="flex items-center gap-x-2">
      <KunChip size="xs" class-name="rounded-md" color="primary">
        {{ categoryText(detail.category) }}
      </KunChip>
      <span
        v-if="detail.lang"
        class="text-default-500 dark:text-default-400 text-xs"
      >
        {{ KUN_GALGAME_OFFICIAL_LANGUAGE_MAP[detail.lang] || detail.lang }}
      </span>
    </div>
    <div
      v-if="detail?.alias.length"
      class="text-default-500 flex flex-wrap gap-2"
    >
      <KunChip
        size="xs"
        color="success"
        v-for="(a, index) in detail.alias"
        :key="index"
      >
        {{ a }}
      </KunChip>
    </div>
  </KunCard>
</template>
