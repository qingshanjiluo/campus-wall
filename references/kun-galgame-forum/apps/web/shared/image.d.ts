// Per-image display metadata from image_service: intrinsic pixel dimensions +
// a base64 ThumbHash. The dims let a surface reserve the image's aspect ratio
// before it loads (no CLS); the ThumbHash drives KunImage's blur-up placeholder.
//
// Global ambient (this is a script .d.ts, no import/export) so every cover
// surface — the home feed, activity cards, topic cards — can type its
// `coverImageMeta` map (keyed by the /image/<hash> cover token) without an
// import. `thumbhash` is optional: empty for images predating the image_service
// thumbhash backfill (dims are always present).
interface KunImageMeta {
  width: number
  height: number
  thumbhash?: string
}
