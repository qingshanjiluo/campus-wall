import type { KunGalgameTagCategory } from '~/constants/galgameTag'
import type { GalgameCard } from './galgame'

export interface GalgameTag {
  id: number
  name: string
  category: KunGalgameTagCategory
}

export interface GalgameTagItem {
  id: number
  name: string
  category: KunGalgameTagCategory
  galgame_count: number
}

// A search hit. The catalog's entity search is a picker feed — identity and
// nothing else — so a hit genuinely does not know its category or how many
// games carry it. Kept distinct from the browse row rather than zero-filled:
// a card that renders "+ 0" for a tag with 600 games is worse than a card that
// says nothing.
export interface GalgameTaxonomySearchItem {
  id: number
  name: string
}

export interface GalgameTagDetail {
  id: number
  name: string
  category: KunGalgameTagCategory
  // The canonical vocabulary's do-not-display tier. Such a tag appears in no
  // list, no search and no picker; its page still renders for a direct link,
  // but carries noindex.
  hidden: boolean
  description: string
  alias: string[]
  galgame: GalgameCard[]
  galgame_count: number
}
