-- 010: galgame_comment edit timestamp
--
-- Sister field to the existing `updated` column. `updated` ticks on
-- every row write (incl. like_count bumps), so it isn't a reliable
-- signal of "the author edited their content". `edited` is set ONLY
-- when the author rewrites the content via the new PUT endpoint, and
-- drives the "已编辑" indicator the UI surfaces — same pattern as
-- topic_reply.edited and galgame_toolset_comment.edited.

BEGIN;

ALTER TABLE galgame_comment
    ADD COLUMN IF NOT EXISTS edited TIMESTAMPTZ NULL;

COMMIT;
