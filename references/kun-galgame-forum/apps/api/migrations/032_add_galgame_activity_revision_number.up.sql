-- Store the per-galgame revision NUMBER (galgame_revision.revision) for
-- GALGAME_EDIT activities, alongside the existing global wiki_revision_id.
-- The edit card's diff endpoint keys on this number
-- (GET /galgame/:gid/revisions/:rev/diff), so persisting it lets the card build
-- the diff URL directly instead of resolving id→number at render time.
--
-- Nullable on purpose: rows synced before the wiki `/galgame/revisions/recent`
-- feed exposed `revision` stay NULL and fall back to the id→number resolution
-- until they age out of the feed.
ALTER TABLE galgame_activity
  ADD COLUMN IF NOT EXISTS wiki_revision_number INTEGER;
