import type { GalgameCard } from './galgame'

export interface GalgameEngine {
  id: number
  name: string
}

export interface GalgameEngineItem {
  id: number
  name: string
  // BE `/galgame-engine` list returns this; the embedded
  // `GalgameDetail.engine[]` (GalgameDetailEngine) does not. Same
  // type is shared by both contexts (Info.vue + list Card), so
  // optional keeps Card.vue's metadata chip on the list view while
  // not lying about the detail-embedded shape.
  description?: string
  alias: string[]
  galgame_count: number
}

export interface GalgameEngineDetail {
  id: number
  name: string
  description: string
  alias: string[]
  galgame: GalgameCard[]
  galgame_count: number
}
