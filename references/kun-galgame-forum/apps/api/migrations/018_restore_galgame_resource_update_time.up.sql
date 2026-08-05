-- 018: restore galgame.resource_update_time as the dedicated content-update
-- sort key.
--
-- Root cause: 000_baseline gave galgame a dedicated `resource_update_time`
-- column, but 005_cleanup_wiki_managed_data DROPPED it together with genuinely
-- wiki-managed columns (name_*, intro_*, vndb_id, ...). Resources are a LOCAL
-- concern (kungal/moyu only), so it should never have been treated as
-- wiki-managed. After that drop, the /galgame list fell back to sorting by the
-- generic audit column `updated` (GORM autoUpdateTime) — so likes / favorites /
-- comments (any row write) reordered the list, and real content updates
-- (publish a resource, merge an edit) didn't move at all. moyu and wiki both
-- already keep a dedicated resource_update_time; this realigns kungal.
--
-- Type is `timestamp(3) without time zone` to match galgame.created / updated
-- (and moyu's column) — deliberately NOT timestamptz, so the table stays
-- internally consistent.
BEGIN;

ALTER TABLE galgame
  ADD COLUMN IF NOT EXISTS resource_update_time timestamp(3) without time zone;

-- Backfill to the real content time = GREATEST(galgame.created, latest resource
-- PUBLISH time). Uses galgame_resource.created (the real publish date — spread
-- over 754 distinct days), NOT galgame_resource.updated (polluted: clustered
-- into ~95 days by a bulk write, so it is not a reliable edit signal).
-- Galgames with no resources fall back to their own created time.
UPDATE galgame g
SET resource_update_time = sub.t
FROM (
  SELECT g2.id,
         GREATEST(g2.created, COALESCE(MAX(r.created), g2.created)) AS t
  FROM galgame g2
  LEFT JOIN galgame_resource r ON r.galgame_id = g2.id
  GROUP BY g2.id, g2.created
) sub
WHERE g.id = sub.id;

-- Every row now has a value → enforce NOT NULL and give new lazy-create stubs a
-- sensible default (GORM also sets it via autoCreateTime on the model).
ALTER TABLE galgame
  ALTER COLUMN resource_update_time SET DEFAULT CURRENT_TIMESTAMP,
  ALTER COLUMN resource_update_time SET NOT NULL;

-- The /galgame list ORDER BYs this column now; 016 only indexed g.created.
CREATE INDEX IF NOT EXISTS idx_galgame_resource_update_time
  ON galgame (resource_update_time DESC);

COMMIT;
