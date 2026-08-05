// P3 retired the PUBLIC series pages because the WIKI's series vocabulary had
// no migration path: 146 entries, 6 of which corresponded to anything in the
// catalog. Half-moving that would have been worse than dropping it.
//
// The public page is back on a different population — the catalog's own
// source-mirrored series facet, the same one works?series_id= filters on and
// every work record's series block points at. That is a complete facet, not the
// half-migration that was refused.
//
// GalgameSeriesSearchItem below still serves the STAFF editor's wiki rows.

import type { GalgameCard } from './galgame'

/** One row of the bare `/galgame-series` lane: the whole facet at once, no
 * samples. The editor's series picker reads this one — it has no search index
 * behind it, so the picker walks the list and filters in the browser. */
export interface GalgameSeriesItem {
  id: number
  name: string
  /** Upstream's member count: the series' whole catalogue, NOT the forum-local
   * subset the page behind the link renders. The other three indexes carry the
   * same number with the same caveat. */
  galgame_count: number
}

/** One member work behind a series card: what the cover montage fans out and
 * what the "包含 …" line names. Not a GalgameCard — the montage needs no
 * views, likes or local enrichment. */
export interface GalgameSeriesSample {
  name: KunLanguage
  effective_banner_hash?: string
  effective_banner_url?: string
  effective_banner_thumbhash?: string
}

/** The rich card, on the series index and on a game's detail page. */
export interface GalgameSeriesCard {
  id: number
  name: string
  /** Read off the SAMPLE, not off the series — the catalog has no series-level
   * content verdict, so this means "at least one of the works shown here is
   * r18", which is what the chip sits next to. */
  is_nsfw: boolean
  galgame_count: number
  sample_galgame: GalgameSeriesSample[]
}

/** A catalog series entity page: identity + the forum-local, filterable subset
 * of its member works — the same shape the tag / official / engine pages carry,
 * so the four render through one set of components. */
export interface GalgameSeriesDetail {
  id: number
  name: string
  /** ONE intro, already chosen server-side: the catalog keeps every source's
   * text for a series, and stacking them under the title reads as a bug. */
  description: string
  galgame: GalgameCard[]
  galgame_count: number
}

// Wiki /series/search and /series/modal return FULL galgame rows
// (snake_case multi-language `name_<locale>` columns plus a bunch of
// other fields the select widget doesn't need). The widget reads
// `id` for the value and runs the names through `galgameNameFromWire`
// to pick the user-preferred locale. Extra wire fields are tolerated
// but unused.
export interface GalgameSeriesSearchItem {
  id: number
  name_en_us?: string
  name_ja_jp?: string
  name_zh_cn?: string
  name_zh_tw?: string
}

/** One series a game belongs to, as it appears on the game's detail page.
 * Identity only: the member count lives on the series page, and a second count
 * next to the link would eventually disagree with it. */
export interface GalgameDetailSeriesRef {
  id: number
  name: string
}
