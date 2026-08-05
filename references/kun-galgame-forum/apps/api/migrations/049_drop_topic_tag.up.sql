-- 049: topic tags are retired. Drop the topic_tag + topic_tag_relation tables
-- and the topic_draft.tags column.
--
-- DEPLOY-THEN-DROP: this migration is EXCLUDED from the default auto-run (see
-- cmd/migrate --exclude). Run it manually via `--only=049` AFTER the tag-free
-- code is deployed, otherwise the still-running pre-deploy code reads a dropped
-- table and topic list/detail/home/search 500 during the deploy window.
-- Idempotent.
BEGIN;

DROP TABLE IF EXISTS topic_tag_relation;
DROP TABLE IF EXISTS topic_tag;
ALTER TABLE topic_draft DROP COLUMN IF EXISTS tags;

COMMIT;
