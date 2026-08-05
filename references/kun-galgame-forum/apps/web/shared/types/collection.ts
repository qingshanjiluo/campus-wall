import type { GalgameCard } from './galgame'

export type CollectionVisibility = 'public' | 'private' | 'restricted'

export interface CollectionUserBrief {
  id: number
  name: string
  avatar: string
}

// One card in a user's collection grid (GET /user/:id/collections).
export interface CollectionSummary {
  id: number
  name: string
  description: string
  visibility: CollectionVisibility
  is_default: boolean
  item_count: number
  preview_covers: string[]
  created: string
  updated: string
}

// GET /galgame/collection/:cid — metadata + one page of games.
export interface CollectionDetail {
  id: number
  name: string
  description: string
  visibility: CollectionVisibility
  is_default: boolean
  item_count: number
  is_owner: boolean
  owner: CollectionUserBrief
  viewers: CollectionUserBrief[]
  galgames: GalgameCard[]
  total: number
  created: string
  updated: string
}

// One row in the collection-picker modal (GET /galgame/:gid/collections/mine).
export interface MyCollectionForGalgame {
  id: number
  name: string
  visibility: CollectionVisibility
  is_default: boolean
  item_count: number
  contains: boolean
}
