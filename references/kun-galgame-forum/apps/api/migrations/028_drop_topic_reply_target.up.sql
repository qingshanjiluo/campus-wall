-- 028: drop topic_reply_target.
--
-- The multi-target reply model was retired in favour of inline @mention / #quote
-- tokens in topic_reply.content. The Phase-4 data migration (run on prod
-- 2026-06-21) folded every topic_reply_target row into the owning reply's content
-- and cleared the table; the read path, model, and lifecycle helpers have since
-- been removed from the code, so nothing reads or writes this table anymore.
--
-- DROP TABLE also drops its owned sequence, primary key, unique index, and the two
-- self-FKs to topic_reply. Irreversible: the rows are gone (the migration's backup
-- tables were already dropped after verification). The down recreates an EMPTY
-- table with the original structure only.
BEGIN;

DROP TABLE IF EXISTS public.topic_reply_target;
DROP SEQUENCE IF EXISTS public.topic_reply_target_id_seq;

COMMIT;
