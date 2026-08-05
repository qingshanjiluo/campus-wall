import { getPreferredLanguageText } from './getPreferredLanguageText'

// Wire-format name resolution, shared so every galgame surface resolves a
// title identically.
//
// The status→badge half of this module retired with the wiki status machine:
// lifecycle is claim_state now, and its presentation lives in
// galgameClaimState.ts. The two vocabularies are not a rename of each other, so
// keeping a translation layer between them would have been a third vocabulary.

// Wire-format galgame rows carry snake_case name_<locale> columns; the
// rest of the site keys KunLanguage by hyphenated locale. Build the
// hyphen shape and run it through the standard locale-priority picker
// so titles resolve identically to GalgameCard / detail pages.
export interface WireGalgameName {
  name_en_us?: string
  name_ja_jp?: string
  name_zh_cn?: string
  name_zh_tw?: string
}

export const galgameNameFromWire = (
  g: WireGalgameName,
  fallback = ''
): string => {
  return (
    getPreferredLanguageText({
      'en-us': g.name_en_us ?? '',
      'ja-jp': g.name_ja_jp ?? '',
      'zh-cn': g.name_zh_cn ?? '',
      'zh-tw': g.name_zh_tw ?? ''
    }) || fallback
  )
}
