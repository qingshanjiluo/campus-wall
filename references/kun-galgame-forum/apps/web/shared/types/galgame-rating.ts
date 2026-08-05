import type { GalgameOfficialItem } from './galgame-official'

export interface GalgameRatingGalgameInfo {
  id: number
  name: KunLanguage
  content_limit: string
  official: GalgameOfficialItem[]
  age_limit: string
  original_language: string
  banner: string
  effective_banner_hash?: string
  effective_banner_url?: string
  // Derived banner's intrinsic metadata for no-CLS aspect-ratio + blur-up.
  effective_banner_width?: number
  effective_banner_height?: number
  effective_banner_thumbhash?: string
  // rating overall average
  rating: number
  rating_count: number
}

export interface GalgameRatingCard {
  id: number
  user: KunUser
  recommend: string
  overall: number
  view: number
  galgame_type: string[]
  play_status: string
  // Author's short writeup; BE returns it on every list row (RatingCard),
  // so listing it here mirrors the wire shape. Card.vue doesn't render it
  // today but consumers like user/Rating.vue may surface a preview later
  // without having to widen the type.
  short_summary: string

  art: number
  story: number
  music: number
  character: number
  route: number
  system: number
  voice: number
  replay_value: number
  spoiler_level: string

  like_count: number
  created: Date | string
  updated: Date | string

  galgame: {
    id: number
    name: KunLanguage
    content_limit: string
  }
}

export interface GalgameRatingDetails extends GalgameRatingCard {
  is_liked: boolean
  liked_users: KunUser[]
  galgame: GalgameRatingGalgameInfo
}

export interface GalgameRatingCardOnGalgamePage extends GalgameRatingCard {
  is_liked: boolean
}
