export type FriendLinkCategory = 'official' | 'galgame' | 'others'

export type FriendLinkStatus = 'normal' | 'essential' | 'down'

export interface FriendLink {
  id: number
  category: FriendLinkCategory
  name: string
  link: string
  description: string
  /** Legacy full image URL (image_service webp, or the legacy /friends/<name>.webp). */
  banner: string
  /** Content-addressed image hash driving the new uploader. */
  banner_image_hash: string
  /** Resolved CDN url for display (falls back to the legacy `banner`). */
  banner_url: string
  status: FriendLinkStatus
  sort_order: number
  created: string
  updated: string
}

/** Response shape of GET /friend-link — links grouped by the 3 fixed categories. */
export interface GroupedFriendLinks {
  official: FriendLink[]
  galgame: FriendLink[]
  others: FriendLink[]
}

/** Create/update payload from the admin form (id present only when editing). */
export interface FriendLinkInput {
  id?: number
  category: FriendLinkCategory
  name: string
  link: string
  description: string
  /** Legacy URL, kept and submitted unchanged so un-migrated rows aren't wiped. */
  banner: string
  /** Content-addressed image hash from the cover uploader. */
  banner_image_hash: string
  status: FriendLinkStatus
}
