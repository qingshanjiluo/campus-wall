-- 030: drop the unmoe ("不萌记录") table — the feature is fully retired
-- (frontend page + nav entry + backend handler/repo/model/route all removed).
--
-- unmoe held read-only violation/translation log rows. Nothing references it: its
-- only FK was unmoe.user_id -> "user".id (dropped with the table), and no other
-- table points at unmoe. The id sequence is OWNED BY unmoe.id, so DROP TABLE
-- reclaims it too. Idempotent.
BEGIN;

DROP TABLE IF EXISTS public.unmoe;

COMMIT;
