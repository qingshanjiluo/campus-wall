<script setup lang="ts">
// Collapses its slotted content to `maxHeight` px; when the content overflows,
// a 显示更多 / 收起 toggle reveals (or re-hides) the rest. Used by the reply /
// comment feed cards (300px) so long bodies don't dominate the feed. Mirrors the
// edit card's diff-collapse, measured with a ResizeObserver so late-loading
// content (images, fonts) re-checks overflow.
const props = withDefaults(defineProps<{ maxHeight?: number }>(), {
  maxHeight: 300
})

const body = ref<HTMLElement | null>(null)
const isExpanded = ref(false)
const isOverflowing = ref(false)
let resizeObserver: ResizeObserver | null = null

const measure = () => {
  isOverflowing.value = !!body.value && body.value.scrollHeight > props.maxHeight
}

const bodyStyle = computed(() =>
  !isOverflowing.value || isExpanded.value
    ? undefined
    : { maxHeight: `${props.maxHeight}px`, overflow: 'hidden' }
)

onMounted(() => {
  if (body.value) {
    resizeObserver = new ResizeObserver(measure)
    resizeObserver.observe(body.value)
  }
  measure()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
})
</script>

<template>
  <div>
    <div ref="body" :style="bodyStyle">
      <slot />
    </div>
    <button
      v-if="isOverflowing"
      type="button"
      class="text-primary mt-1 flex items-center gap-1 text-sm"
      @click="isExpanded = !isExpanded"
    >
      {{ isExpanded ? '收起' : '显示更多' }}
      <KunIcon
        :name="isExpanded ? 'lucide:chevron-up' : 'lucide:chevron-down'"
        class="size-4"
      />
    </button>
  </div>
</template>
