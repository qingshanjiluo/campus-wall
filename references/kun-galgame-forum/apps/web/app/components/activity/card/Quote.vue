<script setup lang="ts">
// A quoted-content block for feed cards: the reply being replied to, or the
// comment being commented on. A rounded vertical bar on the left, no background.
// Optional label (e.g. "#5") prefixes the clamped content.
defineProps<{ content: string; label?: string }>()
</script>

<template>
  <div class="text-default-500 flex gap-2 text-sm">
    <div class="bg-default-300 w-1 shrink-0 rounded-full" />
    <div class="min-w-0">
      <span v-if="label" class="text-default-400 mr-1 text-xs font-medium">
        {{ label }}
      </span>
      <span class="text-default-500 line-clamp-2 break-all">
        {{ markdownToText(content) }}
      </span>
    </div>
  </div>
</template>
