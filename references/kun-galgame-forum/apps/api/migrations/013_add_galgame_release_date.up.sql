-- 013: Mirror wiki's release_date onto the local galgame stats table.
--
-- WHY a local copy at all: kungal's GET /galgame browse list does NOT
-- proxy wiki's /galgame. It filters + sorts on the LOCAL `galgame` table
-- (the curated ~5.8k community subset, stats-only) and only THEN batch-
-- hydrates display metadata from wiki. So to offer "filter by release
-- year / month" (wiki §17) at the list level, the release_date must be
-- present at the layer where the WHERE/ORDER runs — i.e. locally.
--
-- release_date is the canonical wiki field (PG `date`, 91% coverage).
-- We store a NULLable mirror; NULL = unknown / not-yet-backfilled. Rows
-- with NULL are auto-excluded once any released_from/to bound is set
-- (PG `>=`/`<=` on NULL → UNKNOWN → dropped), matching wiki §17.4.
--
-- POPULATION: `cmd/backfill-release-date` pulls release_date from wiki
-- for every local id and is idempotent — re-running is the refresh
-- mechanism (release_date is effectively immutable once a title ships,
-- so periodic re-runs cover the rare wiki-side edit).

BEGIN;

ALTER TABLE galgame
  ADD COLUMN IF NOT EXISTS release_date DATE;

-- btree over a DATE column serves both range predicates
-- (release_date >= ? AND release_date <= ?) and ORDER BY release_date.
-- Partial-on-NOT-NULL would be marginally smaller but range scans on the
-- full index are fine at this table size and keep ORDER BY NULLS handling
-- simple.
CREATE INDEX IF NOT EXISTS idx_galgame_release_date
  ON galgame (release_date);

COMMIT;
