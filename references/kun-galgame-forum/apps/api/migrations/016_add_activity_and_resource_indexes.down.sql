-- 016 down: drop the home/activity hot-path indexes.
-- Pure performance indexes — dropping loses no data, only reverts to seq scans.

BEGIN;

DROP INDEX IF EXISTS idx_galgame_resource_galgame_id;
DROP INDEX IF EXISTS idx_message_type_created;

DROP INDEX IF EXISTS idx_topic_created;
DROP INDEX IF EXISTS idx_topic_reply_created;
DROP INDEX IF EXISTS idx_topic_comment_created;
DROP INDEX IF EXISTS idx_galgame_created;
DROP INDEX IF EXISTS idx_galgame_comment_created;
DROP INDEX IF EXISTS idx_galgame_resource_created;
DROP INDEX IF EXISTS idx_galgame_rating_created;
DROP INDEX IF EXISTS idx_galgame_rating_comment_created;
DROP INDEX IF EXISTS idx_galgame_website_created;
DROP INDEX IF EXISTS idx_galgame_website_comment_created;
DROP INDEX IF EXISTS idx_galgame_toolset_created;
DROP INDEX IF EXISTS idx_galgame_toolset_resource_created;
DROP INDEX IF EXISTS idx_galgame_toolset_comment_created;
DROP INDEX IF EXISTS idx_todo_created;
DROP INDEX IF EXISTS idx_update_log_created;

COMMIT;
