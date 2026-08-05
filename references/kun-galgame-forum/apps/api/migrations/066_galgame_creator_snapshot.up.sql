-- A2-3 re-anchoring: freeze the wiki-era creator on the local galgame row.
--
-- kungal renders an author chip on every galgame card (list, ranking, RSS,
-- profile tabs). That user id came from the wiki's own row, which the catalog
-- re-anchoring removes as a source: the catalog is a cross-source registry of
-- WORKS and deliberately carries no product's submitter (doc 106 R2 — wiki
-- state-machine fields never cross into the public catalog contract).
--
-- The wiki-era attribution is real history, so it is frozen here rather than
-- dropped: one column, backfilled once from the surviving /internal ownership
-- meta op (cmd/backfill-galgame-creator), never written again on the read path.
-- NULL = no wiki-era creator known (a catalog work kungal never ingested, or a
-- row the backfill has not reached), which the card renders as no author chip.
ALTER TABLE galgame ADD COLUMN IF NOT EXISTS creator_user_id integer;

COMMENT ON COLUMN galgame.creator_user_id IS
  'Frozen wiki-era submitter (A2-3). Backfilled once from /internal/galgame/meta; NULL = unknown.';
