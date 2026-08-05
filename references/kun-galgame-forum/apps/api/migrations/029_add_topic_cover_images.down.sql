-- 029 down: drop topic.cover_images. The stored cover tokens are discarded —
-- this only reverts the schema change.
BEGIN;

ALTER TABLE public.topic
  DROP COLUMN IF EXISTS cover_images;

COMMIT;
