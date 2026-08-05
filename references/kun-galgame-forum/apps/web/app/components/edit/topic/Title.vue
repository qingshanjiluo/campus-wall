<script setup lang="ts">
import { useTopicEditorStore } from '~/composables/topic/useTopicEditorStore'
import { KUN_TOPIC_TITLE_LENGTH_LIMIT } from '~/config/limit'

// A borderless, shadow-less title — it lives inside the shared document card
// (edit/topic/Layout.vue), separated from the body by a faint divider, so it
// must not draw its own border/shadow. A native input keeps it fully unstyled
// (KunInput always renders the unified border).
const { title } = useTopicEditorStore()
</script>

<template>
  <input
    v-model="title"
    type="text"
    :maxlength="KUN_TOPIC_TITLE_LENGTH_LIMIT"
    placeholder="请在标题中点名您话题的主旨"
    class="placeholder:text-default-400 w-full border-none bg-transparent text-2xl font-bold outline-none"
  />
</template>
