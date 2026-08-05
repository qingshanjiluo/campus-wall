-- Content-addressed cover/banner/icon hashes (image_service) for the forum's
-- own entities. The legacy free-form URL columns (banner / icon) are KEPT for
-- fallback + rollback during the transition; once the backfill is verified in
-- prod, a later migration can drop them. Read side prefers *_image_hash and
-- falls back to the legacy URL when the hash is empty.

ALTER TABLE doc_article
  ADD COLUMN IF NOT EXISTS banner_image_hash VARCHAR(128) NOT NULL DEFAULT '';

ALTER TABLE doc_category
  ADD COLUMN IF NOT EXISTS icon_image_hash VARCHAR(128) NOT NULL DEFAULT '';

ALTER TABLE friend_link
  ADD COLUMN IF NOT EXISTS banner_image_hash VARCHAR(128) NOT NULL DEFAULT '';

ALTER TABLE galgame_website
  ADD COLUMN IF NOT EXISTS icon_image_hash VARCHAR(128) NOT NULL DEFAULT '';
