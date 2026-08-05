import type {
  KunEditorAdapters,
  MentionUser,
  StickerPack
} from '@kungal/editor-core'
import { stickerArray } from '~/constants/sticker'

// Forum POLICY for the extracted <KunEditor> (@kungal/editor-*). The editor ships
// the MECHANISM (ProseMirror nodes / keymaps / serialization); these adapters
// inject the forum's policy: where image uploads go (`/image/topic`), how
// @mentions resolve (`/user/search`), which stickers exist, how toasts show
// (`useMessage`). See docs/oauth + the library's docs/INTEGRATION.md.
//
// `image: false` (the galgame 简介 editor) drops the upload + sticker adapters
// together — the library's "image-free editor" is simply an omitted `uploadImage`
// (stickers render as image nodes), which replaces the old `disableImage` flag.
export const useKunEditorAdapters = (opts?: {
  image?: boolean
}): KunEditorAdapters => {
  const allowImage = opts?.image !== false

  const uploadImage = async (file: File) => {
    const form = new FormData()
    form.append('image', file)
    const url = await kunFetch<string>('/image/topic', {
      method: 'POST',
      body: form,
      watch: false
    })
    // The adapter contract wants a rejection to abort the image; kunFetch yields
    // null on a failed request.
    if (!url) {
      throw new Error('图片上传失败')
    }
    return url
  }

  // The forum's /user/search already returns the library's MentionUser shape
  // ({ id, name, avatar? } — it mirrors the Go dto.MentionUser). Debouncing is
  // handled inside the editor, so this is a bare fetch.
  const searchMentionUsers = async (query: string): Promise<MentionUser[]> =>
    (await kunFetch<MentionUser[]>('/user/search', {
      query: { q: query, limit: 8 }
    })) ?? []

  // The forum's stickers are one flat pack of image URLs (sticker.kungal.com),
  // shared with the chat picker via _stickers.ts.
  const stickerSource = (): StickerPack[] => [
    { name: 'KUNgal', stickers: stickerArray.map((src) => ({ src, name: src })) }
  ]

  // KunMessageType is identical to the library's NotifyLevel
  // ('info' | 'success' | 'warn' | 'error'), so no mapping is needed.
  const notify: KunEditorAdapters['notify'] = (message, level) => {
    useMessage(message, level)
  }

  return allowImage
    ? { uploadImage, searchMentionUsers, stickerSource, notify }
    : { searchMentionUsers, notify }
}
