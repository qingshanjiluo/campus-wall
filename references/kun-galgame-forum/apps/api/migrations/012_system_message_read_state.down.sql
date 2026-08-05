-- 012 down: Best-effort schema rollback.
--
-- Per-user cursors cannot reconstruct the old global `status` flag (it's a
-- one-way data-shape change). We restore the column shape so old code can
-- still compile/query, defaulting to 'unread' for every existing row —
-- callers must treat the value as informational only after a rollback.

BEGIN;

ALTER TABLE system_message
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'unread';

DROP TABLE IF EXISTS system_message_read_state;

COMMIT;
