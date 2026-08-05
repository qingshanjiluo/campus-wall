export interface WebsiteCategory {
  id: number
  name: string
  label: string
  description: string
}

export interface WebsiteTag {
  id: number
  name: string
  description: string
  label: string
  level: number
}

export interface WebsiteTagDetail {
  id: number
  name: string
  label: string
  level: number
  description: string
  website_count: number
  websites: WebsiteCard[]
  created: Date | string
  updated: Date | string
}

export interface WebsiteCategoryDetail {
  id: number
  name: string
  label: string
  description: string
  website_count: number
  websites: WebsiteCard[]
  created: Date | string
  updated: Date | string
}

export interface WebsiteCard {
  id: number
  name: string
  description: string
  domain: string
  age_limit: string
  level: number
  /** Legacy icon URL. */
  icon: string
  /** Content-addressed image hash. */
  icon_image_hash: string
  /** Resolved CDN url for display (falls back to the legacy `icon`). */
  icon_url: string
  price: number
  category: string
}

export interface WebsiteDetail {
  id: number
  name: string
  url: string
  description: string
  /** Legacy icon URL. */
  icon: string
  /** Content-addressed image hash. */
  icon_image_hash: string
  /** Resolved CDN url for display (falls back to the legacy `icon`). */
  icon_url: string
  view: number
  language: string
  age_limit: 'all' | 'r18'
  category: WebsiteCategory
  tags: WebsiteTag[]
  like_count: number
  is_liked: boolean
  favorite_count: number
  is_favorited: boolean
  domain: string[]
  create_time: string
  comment: {
    id: number
    content: string
    user: KunUser
    created: Date | string
    updated: Date | string
  }[]

  created: Date | string
  updated: Date | string
}
