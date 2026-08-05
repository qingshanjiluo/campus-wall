export type DocEditorMode = 'create' | 'rewrite'

export interface DocEditorForm {
  article_id: number | null
  title: string
  slug: string
  description: string
  // Legacy full-URL cover, submitted unchanged so editing a not-yet-migrated
  // doc without touching the cover never wipes its existing URL.
  banner: string
  // Content-addressed cover hash (the value managed by KunCoverUpload).
  banner_image_hash: string
  status: number
  is_pin: boolean
  content_markdown: string
  category_id: number
  tag_ids: number[]
}
