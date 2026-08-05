-- 044: retire galgame_comment.target_user_id — the legacy "评论给 xxx" (comment
-- directed at a specific user). Notifications now come solely from the editor's
-- @-mention (inline tokens in content, parsed by markdown.ExtractMentionIDs →
-- a "mentioned" message), mirroring the topic reply-target retirement (028).
--
-- The read path, request/response fields, the "被评论" profile tab, and the
-- per-target notification/reward were all removed in code first; this drops the
-- now-unused column. galgame_rating_comment.target_user_id is a separate feature
-- and is intentionally left in place. Idempotent.
BEGIN;

ALTER TABLE public.galgame_comment DROP COLUMN IF EXISTS target_user_id;

COMMIT;
