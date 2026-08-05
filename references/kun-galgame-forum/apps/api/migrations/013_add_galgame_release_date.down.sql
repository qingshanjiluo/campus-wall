-- 013 down: drop the local release_date mirror + its index.
--
-- Pure mirror of wiki data, so dropping loses nothing authoritative —
-- re-running the migration + backfill reconstructs it.

BEGIN;

DROP INDEX IF EXISTS idx_galgame_release_date;

ALTER TABLE galgame
  DROP COLUMN IF EXISTS release_date;

COMMIT;
