import type { KunGalgameOfficialCategory } from '~/constants/galgameOfficial'
import type { GalgameCard } from './galgame'

export interface GalgameOfficial {
  id: number
  name: string
  link: string
  category: KunGalgameOfficialCategory
  lang: string
}

export interface GalgameOfficialItem {
  id: number
  name: string
  link: string
  category: KunGalgameOfficialCategory
  /** What this label DID on the work it is attached to. Distinct from
   * `category`, which is what kind of organisation it is — one brand can be
   * both developer and publisher of the same title.
   *
   * Optional because this shape is shared with the browse lanes, where a label
   * is not attached to any work and the endpoint sends no roles at all. Present
   * on the galgame detail payload. */
  roles?: string[]
  lang: string
  alias: string[]
  galgame_count: number
}

// One of a 会社's web presences. `source` is the catalog's own key
// (official_site / twitter / cien) — it NAMES the link, so an X account is
// never rendered as "官方网站".
export interface GalgameOfficialLink {
  source: string
  url: string
}

export interface GalgameOfficialDetail {
  id: number
  name: string
  // Original-language name (wiki PR4 sub-change, K-PR6). BE returns ""
  // when wiki hasn't recorded an original yet. The edit modal pre-fills
  // from this so admins can see the existing value instead of starting
  // from empty.
  original: string
  // `link` is the official site alone (empty when the 会社 has none); `links`
  // is every presence the catalog carries, each labelled by its source.
  links: GalgameOfficialLink[]
  link: string
  category: KunGalgameOfficialCategory
  lang: string
  description: string
  alias: string[]
  galgame: GalgameCard[]
  galgame_count: number
  // Set — and alone — when this label id was merged away in the catalog: the
  // identity lives on that id now and the page 301s there in one hop. Absent on
  // a live label, so `if (data.moved_to)` is the whole check.
  moved_to?: number
}
