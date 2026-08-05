-- Trust & Safety enforcement (Phase 2).
--
-- trust_disposition_applied makes callback delivery idempotent: disposition_id
-- is the dedup key (a replayed callback is a no-op). Matters most for warn_user
-- (hide/remove are already idempotent set-status / delete).
CREATE TABLE IF NOT EXISTS trust_disposition_applied (
    disposition_id bigint PRIMARY KEY,
    action         smallint    NOT NULL,
    applied_at     timestamptz NOT NULL DEFAULT now()
);

-- Extend topic's existing status convention (0=normal, 1=hidden) to the three
-- reportable content types that lacked any soft-hide, so a `hide` disposition
-- soft-hides them (reversible / filtered at render) while `remove` hard-deletes.
ALTER TABLE topic_reply ADD COLUMN IF NOT EXISTS status smallint NOT NULL DEFAULT 0;
ALTER TABLE topic_comment ADD COLUMN IF NOT EXISTS status smallint NOT NULL DEFAULT 0;
ALTER TABLE galgame_comment ADD COLUMN IF NOT EXISTS status smallint NOT NULL DEFAULT 0;
