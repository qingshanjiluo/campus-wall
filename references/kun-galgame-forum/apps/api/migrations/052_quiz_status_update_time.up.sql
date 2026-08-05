-- Repair the galgame_quiz feed trigger from migration 051: galgame_quiz.id is
-- BIGINT, so the uncast NEW.id/OLD.id can't resolve feed_upsert/feed_delete (INT
-- params). The backfill INSERT in 051 coerced fine, so this only bites when the
-- trigger FIRES — e.g. the UPDATE below, or any quiz answer/edit. Cast to integer.
-- Idempotent; harmless where 051 already shipped the fixed version.
CREATE OR REPLACE FUNCTION feed_sync_galgame_quiz() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_QUIZ_CREATION', OLD.id::integer); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_QUIZ_CREATION', NEW.id::integer, NEW.user_id, 0, NEW.question, '/galgame-quiz/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- galgame_quiz gains a last-activity time (status_update_time), mirroring topic:
-- it defaults to the creation time and is bumped whenever a user answers the quiz
-- or the author edits it, so the 题库 list can sort by "most recently active".
-- Idempotent.
ALTER TABLE galgame_quiz ADD COLUMN IF NOT EXISTS status_update_time TIMESTAMPTZ;
UPDATE galgame_quiz SET status_update_time = created WHERE status_update_time IS NULL;
ALTER TABLE galgame_quiz ALTER COLUMN status_update_time SET DEFAULT now();
ALTER TABLE galgame_quiz ALTER COLUMN status_update_time SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_galgame_quiz_status_update_time
  ON galgame_quiz (status_update_time DESC, id DESC);
