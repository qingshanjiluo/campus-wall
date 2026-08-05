-- The doc_category cover migration was reverted: no admin form ever set it and
-- nothing renders a category icon, so the icon_image_hash column added by
-- migration 040 is unused. Drop it. Safe to run anytime — no code references it.
ALTER TABLE doc_category DROP COLUMN IF EXISTS icon_image_hash;
