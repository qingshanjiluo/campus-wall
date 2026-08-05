export interface RankingUserItem {
  id: number
  name: string
  avatar: string
  bio: string
  sort_field: string
  value: number
}

export interface RankingTopicItem {
  id: number
  title: string
  user: KunUser
  sort_field: string
  value: number
}

export interface RankingGalgameItem {
  id: number
  name: KunLanguage
  user: KunUser
  banner: string
  // U2 banner pair — Ranking/Galgame.vue calls getEffectiveBanner()
  // which checks these before falling back to legacy `banner`.
  effective_banner_hash?: string
  effective_banner_url?: string
  sort_field: string
  value: number
}
