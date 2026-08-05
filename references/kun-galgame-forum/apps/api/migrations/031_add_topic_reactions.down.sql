-- Down for 031: drop the unified reaction tables. The legacy topic_like /
-- topic_dislike / topic_reply_like / topic_reply_dislike tables were left intact
-- by the up migration, so reverting simply removes the new tables.
BEGIN;

DROP TABLE IF EXISTS public.topic_reply_reaction;
DROP TABLE IF EXISTS public.topic_reaction;

COMMIT;
