-- 060: drop the five frozen legacy comment tables (charter step 06a finale).
--
-- The galgame / rating / website / toolset comment areas were migrated onto the
-- infra community primitive (steps 03-09); these tables have been FROZEN (no new
-- writes) since the cutover and every reader was re-pointed off them. This drops
-- them for good, plus the four legacy feed trigger functions that referenced them
-- (their triggers die with the tables; the FUNCTIONs linger otherwise — see
-- migrations 034 + 056).
--
-- NOT dropped (deliberately): galgame_comment_community_map, galgame_post_like,
-- resource_comment_community_map — the LIVE community BFF tables (deep-link map,
-- like footprint) that survive the retirement.
--
-- PREREQUISITE / DEPLOY-THEN-DROP: this migration is EXCLUDED from the default
-- auto-run (see cmd/migrate --exclude). Run it MANUALLY via `--only=060` AFTER
-- this wave's code (which no longer touches these tables) is deployed and an
-- observation window has passed. Running it before the pre-deploy code drains
-- would 500 any straggler read/write of a dropped table.
--
-- Idempotent (IF EXISTS throughout).
BEGIN;

-- galgame_comment_like FKs galgame_comment(id) ON DELETE CASCADE, so drop the
-- like table first. The comment tables' self-referencing parent/root FKs drop
-- with their own table; no SURVIVING table references any of the five.
DROP TABLE IF EXISTS galgame_comment_like;
DROP TABLE IF EXISTS galgame_comment;
DROP TABLE IF EXISTS galgame_rating_comment;
DROP TABLE IF EXISTS galgame_website_comment;
DROP TABLE IF EXISTS galgame_toolset_comment;

-- Legacy feed_activity trigger functions (migrations 034 / 056). The triggers
-- were dropped with their tables above; drop the orphaned functions too.
DROP FUNCTION IF EXISTS feed_sync_galgame_comment();
DROP FUNCTION IF EXISTS feed_sync_galgame_rating_comment();
DROP FUNCTION IF EXISTS feed_sync_galgame_website_comment();
DROP FUNCTION IF EXISTS feed_sync_galgame_toolset_comment();

COMMIT;
