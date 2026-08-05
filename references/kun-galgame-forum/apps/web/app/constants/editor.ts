import type { KunToolbarItem } from '@kungal/editor-vue'

// Shared <KunEditorToolbar> button order for EVERY editor in the forum (the
// topic-edit page + the reply/comment/toolset shim), so the toolbar layout is
// consistent site-wide — the reason @kungal/editor-nuxt added the `:items` prop
// (0.21.0). The built-in 'image' button is intentionally ABSENT: every image-
// capable editor renders the host <KunMilkdownImageDialog> beside the toolbar
// instead (URL insert / multi-upload / recent history). `'|'` = divider;
// 'picker' (sticker) auto-drops without its feature and the dividers collapse.
export const KUN_EDITOR_TOOLBAR_ITEMS: KunToolbarItem[] = [
  'heading',
  '|',
  'picker',
  '|',
  'bold',
  'italic',
  'strike',
  'code',
  'link',
  '|',
  'bulletList',
  'orderedList',
  'quote',
  'codeBlock',
  'hr',
  '|',
  'spoiler'
]
