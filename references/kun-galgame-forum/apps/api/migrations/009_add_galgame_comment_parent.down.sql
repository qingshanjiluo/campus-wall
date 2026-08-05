-- 009 down: drop nesting columns + indexes.

BEGIN;

DROP INDEX IF EXISTS idx_galgame_comment_root_created;
DROP INDEX IF EXISTS idx_galgame_comment_parent;

ALTER TABLE galgame_comment
    DROP COLUMN IF EXISTS root_comment_id,
    DROP COLUMN IF EXISTS parent_comment_id;

COMMIT;
