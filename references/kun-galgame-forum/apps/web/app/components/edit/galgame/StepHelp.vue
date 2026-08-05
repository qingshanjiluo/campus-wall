<script setup lang="ts">
// Collapsible help block for the publish stepper. Long explanatory copy
// (NSFW definition, where to source intros, etc.) used to sit as walls of
// gray text inline in the form, which is what made the page feel messy.
// Wrap it here so it's discoverable but collapsed by default.
//
// Uses the project's own useKunDisclosure composable for consistency
// (there's no KunCollapse component in the KunUI set).
withDefaults(defineProps<{ title?: string }>(), {
  title: '填写建议'
})

const { isOpen, onOpenChange } = useKunDisclosure()
</script>

<template>
  <div class="border-default-200 rounded-lg border">
    <button
      type="button"
      class="text-default-600 hover:text-default-900 flex w-full items-center justify-between gap-2 px-3 py-2 text-sm transition-colors"
      @click="onOpenChange"
    >
      <span class="flex items-center gap-1.5">
        <KunIcon name="lucide:info" />
        {{ title }}
      </span>
      <KunIcon
        name="lucide:chevron-down"
        :class="`transition-transform ${isOpen ? 'rotate-180' : ''}`"
      />
    </button>
    <div
      v-show="isOpen"
      class="text-default-500 border-default-200 space-y-2 border-t px-3 py-3 text-sm leading-relaxed"
    >
      <slot />
    </div>
  </div>
</template>
