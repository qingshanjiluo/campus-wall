<script setup lang="ts">
// Image-free markdown editor for the galgame intro fields, wired into the
// schema editor via the editkit `component` escape hatch. `disableImage` is the
// editor library's named "galgame 简介" case — it omits the uploadImage adapter,
// so the toolbar image button, paste/drop upload and stickers are all gone. The
// backend is the authoritative guard: intronorm.StripImages removes any image
// markdown from intros on every write path, so intros are image-free even if a
// value arrives with one. Bridges editkit's modelValue/update:modelValue to the
// forum editor's markdown surface (valueMarkdown / @set-markdown).
const props = defineProps<{ modelValue: unknown; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
</script>

<template>
  <KunMilkdownDualEditorProvider
    :value-markdown="String(props.modelValue ?? '')"
    :disable-image="true"
    placeholder="填写游戏简介（支持 Markdown，不支持图片）"
    @set-markdown="(value) => emit('update:modelValue', value)"
  />
</template>
