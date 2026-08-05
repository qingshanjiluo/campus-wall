-- Revert the galgame comment migration scaffolding (charter step 03).
--
-- galgame_comment_community_map is ALSO created by the infra import tool with
-- the same DDL; dropping it here removes the deep-link map. Only run this down
-- migration before any real cutover — after cutover both tables hold live data
-- (the map + the migrated likes) and dropping them loses it.
DROP INDEX IF EXISTS idx_galgame_post_like_post;
DROP TABLE IF EXISTS galgame_post_like;
DROP TABLE IF EXISTS galgame_comment_community_map;
