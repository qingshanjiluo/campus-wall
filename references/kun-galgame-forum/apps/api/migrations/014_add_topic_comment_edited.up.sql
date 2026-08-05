-- 014: topic_comment edit timestamp
--
-- Sister field to the existing `updated` column. `updated` ticks on
-- every row write (incl. like_count-driven writes), so it isn't a
-- reliable signal of "the author edited their content". `edited` is set
-- ONLY when the author rewrites the content via the new PUT endpoint, and
-- drives the "(编辑于 …)" indicator the UI surfaces — same pattern as
-- topic_reply.edited and galgame_comment.edited (migration 010).

BEGIN;

ALTER TABLE topic_comment
    ADD COLUMN IF NOT EXISTS edited TIMESTAMPTZ NULL;

COMMIT;
