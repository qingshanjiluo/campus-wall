// The single source for what a taxonomy URL looks like TODAY (doc 146).
//
// Every redirect target — the retired `/galgame-{family}` shells resolved in
// server/middleware/legacy-taxonomy.ts, and the merged-id hop a detail page
// issues when the catalog answers "this id moved" (doc 148) — is built from
// these two functions. That is what structurally rules out a redirect chain:
// no caller can name an intermediate form even by accident.
//
// They live in shared/ rather than server/utils because both sides need them:
// Nitro middleware redirects retired URLs, and the detail PAGES redirect a
// merged catalog id, which is a decision that can only be made after the data
// fetch — i.e. inside the page, on both SSR and client navigation.

export const TAXONOMY_FAMILIES = ['tag', 'official', 'engine'] as const
export type TaxonomyFamily = (typeof TAXONOMY_FAMILIES)[number]

export const taxonomyIndexPath = (family: TaxonomyFamily) => `/galgame/${family}`

export const taxonomyDetailPath = (family: TaxonomyFamily, catalogId: number) =>
  `/galgame/${family}/${catalogId}`
