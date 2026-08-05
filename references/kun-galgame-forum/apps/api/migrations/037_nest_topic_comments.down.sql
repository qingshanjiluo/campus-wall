DROP INDEX IF EXISTS idx_topic_comment_parent;
ALTER TABLE topic_comment DROP CONSTRAINT IF EXISTS fk_topic_comment_parent;
ALTER TABLE topic_comment DROP COLUMN IF EXISTS parent_comment_id;
