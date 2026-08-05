-- 025: widen galgame_resource.note from varchar(1007) to varchar(10000) so the
-- resource 备注 can hold rich Markdown (including uploaded image URLs).
--
-- The note used to be a plain-text <textarea> capped at 1007 chars. It now uses
-- the same Milkdown editor as topics / comments and renders through the shared
-- markdown.Render() pipeline (render-on-read in resource_mapper — deliberately
-- no _html column, mirroring how topics store only raw markdown). A single
-- uploaded image is ~120 chars
-- (![alt](https://image.kungal.iloveren.link/aa/bb/<64-hex>_variant.webp)), so
-- 1007 truncated mid-URL and broke images. 10000 leaves room for several images
-- plus formatting while staying bounded (the DTO validators enforce max=10000).
--
-- varchar(1007) -> varchar(10000) only relaxes the length check; Postgres does
-- NOT rewrite the table for a widening, so this is fast and safe on prod.
BEGIN;

ALTER TABLE galgame_resource
  ALTER COLUMN note TYPE varchar(10000);

COMMIT;
