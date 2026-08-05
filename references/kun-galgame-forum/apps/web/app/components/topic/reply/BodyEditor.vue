<script setup lang="ts">
const model = defineModel<string>({ required: true })

const tempReplyStore = useTempReplyStore()
const { pendingQuote } = storeToRefs(tempReplyStore)

const saveMarkdown = (editorMarkdown: string) => {
  model.value = editorMarkdown
}
</script>

<template>
  <!-- Double the editor's default (empty) height for the reply box — the var is
       inherited down to the WYSIWYG surface (see styles/editor/kun-editor.css). -->
  <div style="--kun-editor-min-height: 16rem">
    <KunMilkdownDualEditorProvider
      :value-markdown="model"
      :pending-quote="pendingQuote"
      @set-markdown="saveMarkdown"
      @quote-inserted="tempReplyStore.clearPendingQuote()"
    />
  </div>
</template>
