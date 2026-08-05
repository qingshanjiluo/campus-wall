-- 010 down: drop the edited column.

BEGIN;

ALTER TABLE galgame_comment
    DROP COLUMN IF EXISTS edited;

COMMIT;
