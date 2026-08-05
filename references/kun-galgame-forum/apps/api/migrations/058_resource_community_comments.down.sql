-- Revert the resource comment migration scaffolding (charter step 07).
--
-- resource_comment_community_map is ALSO created by the infra import tool with
-- the same DDL; dropping it here removes the audit map. Only run this down
-- migration before any real cutover — after cutover the table holds live data
-- (the source→community post map) and dropping it loses the retirement audit
-- trail (charter ruling 24).
DROP TABLE IF EXISTS resource_comment_community_map;
