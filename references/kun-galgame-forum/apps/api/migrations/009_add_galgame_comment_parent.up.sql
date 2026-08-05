-- 009: galgame_comment nesting — add parent / root pointers
--
-- Adds the missing reply-graph columns so the detail page can render a
-- threaded view. The legacy schema only tracked `target_user_id` (who
-- you @-mention), which is insufficient to reconstruct "who is replying
-- to which specific comment".
--
-- Backfill strategy: NONE. Pre-existing rows keep both new columns NULL,
-- which the application interprets as "top-level / root comment". This
-- is the only honest migration — `target_user_id` alone cannot
-- reconstruct true parentage (same target user may have posted multiple
-- comments; we cannot guess which one a reply meant). New comments
-- created after this migration carry real parent / root references.
--
-- See docs/migration/comment-nesting/ if added later.

BEGIN;

ALTER TABLE galgame_comment
    ADD COLUMN IF NOT EXISTS parent_comment_id INT NULL
        REFERENCES galgame_comment(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS root_comment_id INT NULL
        REFERENCES galgame_comment(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_galgame_comment_parent
    ON galgame_comment(parent_comment_id);

-- Composite: thread fetch always orders by created within a root.
CREATE INDEX IF NOT EXISTS idx_galgame_comment_root_created
    ON galgame_comment(root_comment_id, created);

COMMIT;
