<script setup lang="ts">
import {
  KUN_GALGAME_TAG_CATEGORY_MAP,
  KUN_GALGAME_TAG_SPOILER_MAP,
  type KunGalgameTagSpoiler
} from '~/constants/galgameTag'

const props = withDefaults(
  defineProps<{
    tags: GalgameDetailTag[]
    // 'mobile' renders the original inline-checkbox filter + larger chips and is
    // mounted right under the page header; 'desktop' keeps the compact popover
    // filter + small chips in the sidebar. Two instances are mounted (one per
    // breakpoint via Galgame.vue) because position, chip size and filter UI all
    // differ between them.
    variant?: 'mobile' | 'desktop'
  }>(),
  { variant: 'desktop' }
)

const isMobile = computed(() => props.variant === 'mobile')

// Adult tags are off by default and only pre-selected when the viewer has NSFW
// enabled (cookie-persisted, so this is correct at SSR setup — same source the
// page's isNsfwMode reads in [gid]/index.vue).
//
// This toggle is now a convenience for NSFW-ENABLED viewers ONLY. The server
// withholds sexual tags from a SFW response entirely (galgame_service.GetDetail
// → withoutSexualTags): hiding them here still shipped them inside the SSR
// payload, where a crawler reads them. So a SFW viewer ticking 成人内容 sees
// nothing appear — there is nothing to reveal, which is the point. Flipping the
// NSFW setting reloads the page (SidebarNSFWToggle), and the detail fetch
// carries the cookie, so the fuller set arrives on that reload; there is
// deliberately no second fetch path for it.
const { showKUNGalgameContentLimit } = storeToRefs(usePersistSettingsStore())
const isNsfwEnabled = computed(
  () =>
    showKUNGalgameContentLimit.value === 'nsfw' ||
    showKUNGalgameContentLimit.value === 'all'
)

// Default filter: everything non-adult (+ 成人内容 when NSFW is on), 无剧透 only.
//
// The category axis is now the CATALOG vocabulary (content / meta, plus the
// reconstructed sexual bucket) rather than the wiki's closed three-value one —
// so this is typed as a plain string set: an upstream vocabulary can grow, and
// a chip whose category we do not recognise must still be filterable rather
// than silently dropped.
const selectedCategories = ref<string[]>(
  isNsfwEnabled.value
    ? ['content', 'meta', 'technical', 'sexual']
    : ['content', 'meta', 'technical']
)
const selectedSpoilerLevels = ref<KunGalgameTagSpoiler[]>([0])

const toggleItemInArray = <T,>(arrayRef: Ref<T[]>, item: T) => {
  const index = arrayRef.value.indexOf(item)
  if (index === -1) {
    arrayRef.value.push(item)
  } else {
    arrayRef.value.splice(index, 1)
  }
}

const toggleCategory = (category: string) => {
  toggleItemInArray(selectedCategories, category)
}

const toggleSpoilerLevel = (spoiler: KunGalgameTagSpoiler) => {
  toggleItemInArray(selectedSpoilerLevels, spoiler)
}

const filteredTags = computed(() => {
  if (
    selectedCategories.value.length === 0 ||
    selectedSpoilerLevels.value.length === 0
  ) {
    return []
  }

  const filtered = props.tags.filter(
    (tag) =>
      selectedCategories.value.includes(tag.category) &&
      selectedSpoilerLevels.value.includes(tag.spoiler_level as 0)
  )
  return filtered.sort((a, b) => a.id - b.id)
})

// Color of the trailing "+N" badge encodes the tag's category. Same
// mapping the leading "#" used to carry before the redesign:
//   content          → primary (blue)
//   sexual           → danger  (red)
//   meta / technical → success (green)
//
// `meta` is the live vocabulary; this only listed `technical` and so painted
// every 作品属性 badge the grey fallback. The catalog re-anchoring replaced the
// wiki's content/sexual/technical axis with a content|meta kind plus a sexual
// flag (catalogTagCategory), so `technical` never arrives any more — it stays
// mapped because rows synthesised from the old wiki data still carry it, and
// both name the same non-content, non-adult bucket.
const countColorByCategory = (category: string): string => {
  if (category === 'content') return 'text-primary'
  if (category === 'sexual') return 'text-danger'
  if (category === 'meta' || category === 'technical') return 'text-success'
  return 'text-default-500'
}
</script>

<template>
  <KunCard
    :is-hoverable="false"
    :is-transparent="false"
    class-name="overflow-visible"
    content-class="space-y-3"
  >
    <!-- Mobile: the original inline-checkbox filter (horizontal scroll), above
         the tag list. The sidebar/desktop variant uses the popover below. -->
    <KunScrollShadow v-if="isMobile" shadow-size="5rem">
      <div class="flex w-fit items-center gap-3 whitespace-nowrap">
        <KunCheckBox
          v-for="(name, key) in KUN_GALGAME_TAG_CATEGORY_MAP"
          :key="key"
          class-name="gap-2"
          :model-value="selectedCategories.includes(key)"
          color="primary"
          @click="toggleCategory(key)"
        >
          {{ name }}
        </KunCheckBox>

        <KunCheckBox
          v-for="(name, key) in KUN_GALGAME_TAG_SPOILER_MAP"
          :key="key"
          class-name="gap-2"
          :model-value="selectedSpoilerLevels.includes(Number(key) as 0)"
          color="primary"
          @click="toggleSpoilerLevel(Number(key) as KunGalgameTagSpoiler)"
        >
          {{ name }}
        </KunCheckBox>
      </div>
    </KunScrollShadow>

    <KunScrollShadow
      axis="vertical"
      shadow-size="3rem"
      :class-name="
        isMobile ? 'max-h-[300px]' : 'max-h-[200px] md:max-h-[400px]'
      "
    >
      <TransitionGroup name="tag-list" tag="div" class="flex flex-wrap gap-1.5">
        <KunLink
          v-for="tag in filteredTags"
          :key="tag.id"
          underline="none"
          :to="`/galgame/tag/${tag.id}`"
        >
          <KunChip
            class-name="bg-default-500/10 cursor-pointer"
            :size="isMobile ? 'md' : 'sm'"
          >
            {{ tag.name }}
            <!-- An unmapped source tag never gets a count from upstream, and
                 no row has one before the supplying wave deploys — both land
                 on 0, which must read as "no badge", not "+0". -->
            <span
              v-if="tag.galgame_count > 0"
              :class="cn('text-xs', countColorByCategory(tag.category))"
            >
              {{ `+${tag.galgame_count}` }}
            </span>
            <span v-if="tag.spoiler_level > 0" class="text-warning-600 text-xs">
              {{ tag.spoiler_level > 1 ? '(严重剧透)' : '(剧透)' }}
            </span>
          </KunChip>
        </KunLink>
      </TransitionGroup>

      <KunNull
        v-if="filteredTags.length === 0"
        description="请至少选择一个类别来查看标签，或调整剧透等级"
      />
    </KunScrollShadow>

    <!-- Desktop: compact popover filter (fits the narrow sidebar column).
         full-width makes the trigger anchor span the card (KunPopover 2.13),
         matching the 查看编辑历史与更新请求 button. -->
    <KunPopover v-if="!isMobile" position="top-start" full-width>
      <template #trigger>
        <KunButton variant="flat" color="primary" size="sm" full-width>
          <KunIcon name="lucide:filter" />
          筛选标签
        </KunButton>
      </template>

      <div class="min-w-[240px] space-y-4 p-4">
        <div class="space-y-2">
          <p class="text-default-500 text-xs font-medium">标签类型</p>
          <div class="flex flex-wrap gap-3">
            <KunCheckBox
              v-for="(name, key) in KUN_GALGAME_TAG_CATEGORY_MAP"
              :key="key"
              class-name="gap-2"
              :model-value="selectedCategories.includes(key)"
              color="primary"
              @click="toggleCategory(key)"
            >
              {{ name }}
            </KunCheckBox>
          </div>
        </div>

        <div class="space-y-2">
          <p class="text-default-500 text-xs font-medium">剧透等级</p>
          <div class="flex flex-wrap gap-3">
            <KunCheckBox
              v-for="(name, key) in KUN_GALGAME_TAG_SPOILER_MAP"
              :key="key"
              class-name="gap-2"
              :model-value="selectedSpoilerLevels.includes(Number(key) as 0)"
              color="primary"
              @click="toggleSpoilerLevel(Number(key) as KunGalgameTagSpoiler)"
            >
              {{ name }}
            </KunCheckBox>
          </div>
        </div>
      </div>
    </KunPopover>
  </KunCard>
</template>

<style scoped>
.tag-list-move,
.tag-list-enter-active,
.tag-list-leave-active {
  transition: all 0.5s cubic-bezier(0.55, 0, 0.1, 1);
}
.tag-list-enter-from,
.tag-list-leave-to {
  opacity: 0;
  transform: scale(0.8);
}
.tag-list-leave-active {
  position: absolute;
}
</style>
