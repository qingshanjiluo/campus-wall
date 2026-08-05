<script setup lang="ts">
// The reply editor (single milkdown body + submit row), extracted so
// TopicReplyPanel can host it in a bottom KunDrawer on mobile and in the fixed
// desktop panel — same logic, two shells. Multi-target tabs were retired: a
// reply is now one body with inline @mention / #quote tokens. A 「引用」 click
// inserts an inline `@author #floor` header into the editor itself (see
// BodyEditor.vue → the milkdown Editor.vue), so the header is WYSIWYG editor content.
const { isReplyRewriting, replyRewrite } = storeToRefs(useTempReplyStore())
const { replyDraft } = storeToRefs(usePersistKUNGalgameReplyStore())

const currentData = computed(() =>
  isReplyRewriting.value ? replyRewrite.value : replyDraft.value
)
</script>

<template>
  <div v-if="currentData" class="space-y-2">
    <!-- Remount on context switch (draft ↔ editing a specific reply) so the
         editor loads the right body. -->
    <TopicReplyBodyEditor
      :key="isReplyRewriting ? `edit-${replyRewrite?.id}` : 'draft'"
      v-model="currentData.mainContent"
    />

    <TopicReplyPanelBtn class="mt-3" />
  </div>
</template>
