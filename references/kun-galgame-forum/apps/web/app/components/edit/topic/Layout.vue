<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import type { KunEditorExpose, KunHeading } from '@kungal/editor-vue'
import { useTopicEditorStore } from '~/composables/topic/useTopicEditorStore'
import { KUN_EDITOR_TOOLBAR_ITEMS } from '~/constants/editor'
// The topic writing surface — a Tencent-Docs-style layout: a sticky formatting
// toolbar pinned at the top, the title below it, then the article body. All the
// publish metadata (category / section / NSFW / covers / read-rules) moved into
// <EditTopicPublishModal>, opened by the 发布话题 button.
//
// The editor's toolbar + view-switch are <KunEditor> scoped slots TELEPORTED
// into the sticky bar: Teleport moves only the DOM, so the slot `api` closures
// still drive the live editor. The toolbar is hidden in Markdown-source mode
// (its commands act on the WYSIWYG doc), tracked via `editorMode`.
const tempStore = useTempEditStore()
const { content } = useTopicEditorStore()
const adapters = useKunEditorAdapters()

// The editor's live heading outline (emitted by <KunEditor> on every change) →
// a faint TOC rail on the left; clicking an entry scrolls the active view (and
// drops the caret) to that heading via the editor's scrollToHeading(index).
const headings = ref<KunHeading[]>([])
const editorRef = ref<KunEditorExpose | null>(null)

const isPublishOpen = ref(false)
const openPublish = () => {
  isPublishOpen.value = true
}

// Drafts (server-side, private) only make sense for a NEW topic — hidden while
// rewriting an existing one (that edits live content, not a draft).
const isDraftOpen = ref(false)

type EditorViewMode = 'wysiwyg' | 'source' | 'split'
const editorMode = ref<EditorViewMode>('wysiwyg')
const onSetMode = (
  mode: EditorViewMode,
  apply: (mode: EditorViewMode) => void
) => {
  editorMode.value = mode
  apply(mode)
}

// Split view is desktop-only — it needs width for two panes and stacks awkwardly
// on a phone, so drop it from the offered view modes below 768px.
const isMobile = useMediaQuery('(max-width: 767px)')
const editorViews = computed<EditorViewMode[]>(() =>
  isMobile.value ? ['wysiwyg', 'source'] : ['wysiwyg', 'source', 'split']
)
// Keep the local mirror in sync with <KunEditor>'s own fallback: when split stops
// being offered (a resize to mobile while in split), it falls back to the first
// offered view (wysiwyg).
watch(isMobile, (mobile) => {
  if (mobile && editorMode.value === 'split') {
    editorMode.value = 'wysiwyg'
  }
})

// Ctrl+Enter opens the publish modal (category / section are chosen there now).
const onKeydown = (event: KeyboardEvent) => {
  if (event.ctrlKey && event.key === 'Enter') {
    event.preventDefault()
    openPublish()
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

onBeforeRouteLeave(async () => {
  // Vue Router 4: return false to stay, anything else to proceed.
  if (tempStore.isTopicRewriting) {
    const res = await useComponentMessageStore().alert(
      '确认离开界面吗？您的更改将不会保存。'
    )
    if (!res) {
      return false
    }
    tempStore.resetRewriteTopicData()
  }
})
</script>

<template>
  <div>
    <!-- Single REAL root box for the page transition — keep this comment INSIDE
         the root (a leading comment would be a second root node).

         Sticky formatting bar. It wraps responsively: on desktop it's one row
         (view-switch · toolbar · 发布); on mobile the toolbar drops to its own
         full-width, horizontally-scrollable row so it never wraps into a tall
         block (view-switch + 发布 stay on the top row). -->
    <div
      class="et-topbar border-default-200 bg-background/90 sticky top-[72px] z-20 mb-4 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border px-3 py-2 backdrop-blur"
    >
      <div id="et-viewswitch" class="shrink-0" />
      <span class="bg-default-200 hidden h-5 w-px shrink-0 sm:block" />
      <KunScrollShadow
        axis="horizontal"
        shadow-size="1.5rem"
        shadow-color="var(--color-default-100)"
        draggable
        class="bg-default-100 order-last w-full rounded-lg px-1 py-1 sm:order-none sm:w-auto sm:min-w-0 sm:flex-1 sm:rounded-none sm:bg-transparent sm:p-0"
      >
        <div id="et-toolbar" class="flex w-max items-center" />
      </KunScrollShadow>
      <div
        class="order-1 ml-auto flex shrink-0 items-center gap-2 sm:order-none sm:ml-0"
      >
        <KunButton
          v-if="!tempStore.isTopicRewriting"
          variant="light"
          color="primary"
          @click="isDraftOpen = true"
        >
          <KunIcon name="lucide:notebook-pen" class="mr-1 h-4 w-4" />
          草稿
        </KunButton>
        <KunButton color="primary" @click="openPublish">
          <KunIcon name="lucide:send" class="mr-1 h-4 w-4" />
          发布话题
        </KunButton>
      </div>
    </div>

    <!-- TOC rail (left) + the document card. The rail is desktop-only (lg+) and
         appears only when the content has headings; it lives in the left blank
         area. The card is a centered reading column (max-w-3xl); in split mode it
         becomes flex-1 and fills the width LEFT of the rail (so the two panes get
         room, with the rail carving out the left). title + body share one
         shadowed surface split by a faint divider; neither draws its own border. -->
    <div class="flex justify-center gap-6">
      <aside v-if="headings.length" class="hidden w-48 shrink-0 lg:block">
        <nav class="sticky top-32 space-y-1 py-1 text-sm">
          <button
            v-for="(h, i) in headings"
            :key="i"
            type="button"
            :style="{ paddingInlineStart: `${(h.level - 1) * 0.75}rem` }"
            :title="h.text"
            class="text-default-400 hover:text-primary-500 block w-full truncate text-left transition-colors"
            @click="editorRef?.scrollToHeading(i)"
          >
            {{ h.text || '(无标题)' }}
          </button>
        </nav>
      </aside>

      <div
        :class="
          cn('min-w-0', editorMode === 'split' ? 'flex-1' : 'w-full max-w-3xl')
        "
      >
        <div class="bg-content1 shadow-kun-md px-6 sm:px-16">
          <div class="border-default-200/60 border-b pt-8 pb-4 sm:pt-14 sm:pb-5">
            <EditTopicTitle />
          </div>

          <div class="pt-5 pb-10 sm:pt-7 sm:pb-20">
            <ClientOnly>
              <KunEditor
                ref="editorRef"
                v-model="content"
                :adapters="adapters"
                :views="editorViews"
                placeholder="在此输入您的话题正文..."
                class="kun-doc-editor"
                @update:headings="headings = $event"
              >
                <template #view-switch="api">
                  <Teleport to="#et-viewswitch" defer>
                    <KunEditorViewSwitch
                      v-bind="api"
                      :set-mode="(mode) => onSetMode(mode, api.setMode)"
                    />
                  </Teleport>
                </template>
                <template #toolbar="api">
                  <Teleport to="#et-toolbar" defer>
                    <template v-if="editorMode === 'wysiwyg'">
                      <KunEditorToolbar v-bind="api" :items="KUN_EDITOR_TOOLBAR_ITEMS" />
                      <span
                        class="bg-default-200 mx-1 h-5 w-px"
                        aria-hidden="true"
                      />
                      <KunMilkdownImageDialog v-bind="api" />
                    </template>
                  </Teleport>
                </template>
              </KunEditor>
            </ClientOnly>
          </div>
        </div>
      </div>
    </div>

    <EditTopicPublishModal v-model="isPublishOpen" />
    <EditTopicDraftModal v-model="isDraftOpen" />
  </div>
</template>

<!-- Global (not scoped): the toolbar is TELEPORTED into #et-toolbar, so a scoped
     rule wouldn't reach it. <KunEditorToolbar>'s root is `flex flex-wrap`; force
     a single row here so #et-toolbar's overflow-x-auto scrolls it instead of it
     stacking into multiple tall rows (the mobile problem). -->
<style>
#et-toolbar > * {
  flex-wrap: nowrap;
}
</style>
