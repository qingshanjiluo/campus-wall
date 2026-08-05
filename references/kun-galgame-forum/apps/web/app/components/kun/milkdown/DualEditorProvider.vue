<script setup lang="ts">
// Migration shim: the forum's editor entry point, now backed by the EXTRACTED
// <KunEditor> (@kungal/editor-nuxt) instead of the in-repo Milkdown editor.
//
// It keeps the EXACT prop/emit/slot surface the 9 call sites already use
// (valueMarkdown / setMarkdown / language / disableImage / pendingQuote + a
// footer slot), so the swap is invisible to them. Forum policy is injected via
// useKunEditorAdapters; the footer chrome (Markdown chip + 字数 + syntax hint)
// lives here because <KunEditor> is headless. The in-repo editor under this
// folder is now unused and will be deleted once runtime parity is verified.
import type { KunEditorExpose } from '@kungal/editor-vue'
import { KUN_EDITOR_TOOLBAR_ITEMS } from '~/constants/editor'
import type { ReplyReference } from '~/store/types/topic/reply'

const props = defineProps<{
  valueMarkdown: string
  language?: Language
  // true removes all image insertion (upload button, paste/drop, sticker) — the
  // library expresses this as an omitted uploadImage adapter. The galgame 简介.
  disableImage?: boolean
  // A 「引用」 signal (reply editor only): insert an inline `@author #floor`.
  pendingQuote?: ReplyReference | null
  // Optional empty-state hint text shown by <KunEditor> when the doc is empty.
  placeholder?: string
}>()

const emits = defineEmits<{
  setMarkdown: [value: string]
  quoteInserted: []
}>()

// Bridge the call sites' one-way `valueMarkdown` + `@setMarkdown` to <KunEditor>'s
// v-model. External replacements flow in via the getter; the editor's own edits
// flow out via the setter.
const model = computed({
  get: () => props.valueMarkdown,
  set: (value) => emits('setMarkdown', value)
})

// disableImage is static per instance (each call site passes a fixed value).
const adapters = useKunEditorAdapters({ image: props.disableImage !== true })

// The quote node is always registered (the in-repo editor always wired the quote
// plugin) so any existing `[#floor](kungal-reply:id)` links round-trip; the
// sticker picker follows the image policy.
const features = { quote: true, sticker: props.disableImage !== true }

const editorRef = ref<KunEditorExpose | null>(null)

// Insert "@author #floor " at the caret via the editor's imperative handle —
// insertMention then insertQuote, each trailing-spaced (the library ports the
// in-repo editor's caret-anchor fix). Replaces the old reactive insertReference.
const insertQuote = (q: ReplyReference) => {
  editorRef.value?.insertMention({ userId: q.userId, name: q.userName })
  editorRef.value?.insertQuote({ refId: String(q.replyId), label: `#${q.floor}` })
  emits('quoteInserted')
}

// Defer a frame so the (freshly painted) editor view is focusable before insert.
const consumePendingQuote = () => {
  const q = props.pendingQuote
  if (q) {
    requestAnimationFrame(() => insertQuote(q))
  }
}

// Live-append: 「引用」 clicked while the editor is already open.
watch(() => props.pendingQuote, consumePendingQuote)
// Fresh-open: 「引用」 set BEFORE this editor mounted — insert once it's ready.
onMounted(consumePendingQuote)

const textCount = computed(() => markdownToText(props.valueMarkdown).length)
</script>

<template>
  <div class="space-y-3">
    <KunEditor
      ref="editorRef"
      v-model="model"
      :adapters="adapters"
      :features="features"
      :locale="language ?? 'zh-cn'"
      :placeholder="placeholder"
      :views="['wysiwyg', 'source']"
    >
      <template #view-switch="api">
        <KunEditorViewSwitch v-bind="api" />
      </template>
      <template #toolbar="api">
        <div class="flex flex-wrap items-center gap-1">
          <KunEditorToolbar v-bind="api" :items="KUN_EDITOR_TOOLBAR_ITEMS" />
          <!-- Same rich image dialog as the topic editor (URL / multi-upload /
               recent history), only when this instance allows images. Clipboard
               paste + drag-drop are handled by <KunEditor> via the adapter. -->
          <template v-if="disableImage !== true">
            <span class="bg-default-200 mx-1 h-5 w-px" aria-hidden="true" />
            <KunMilkdownImageDialog v-bind="api" />
          </template>
        </div>
      </template>
    </KunEditor>

    <div class="flex items-center justify-between text-sm">
      <slot />

      <div class="flex shrink-0 items-center gap-2">
        <KunChip color="success">
          <KunIcon
            name="simple-icons:markdown"
            class="text-success-700 dark:text-success"
          />
          Markdown 支持
        </KunChip>
        <span>
          {{ `${textCount} 字` }}
        </span>
      </div>
    </div>

    <div class="text-default-500 text-sm">
      特殊语法: 您可以使用 ||隐藏文本|| 来隐藏图片或者文字 (目前依然禁止 R18
      图片内容)
    </div>
  </div>
</template>
