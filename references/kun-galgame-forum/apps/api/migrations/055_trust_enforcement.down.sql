ALTER TABLE galgame_comment DROP COLUMN IF EXISTS status;
ALTER TABLE topic_comment DROP COLUMN IF EXISTS status;
ALTER TABLE topic_reply DROP COLUMN IF EXISTS status;
DROP TABLE IF EXISTS trust_disposition_applied;
