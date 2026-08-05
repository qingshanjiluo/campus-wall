-- Feed 出题 (galgame quiz) creations into the materialized activity feed
-- (migration 034): one GALGAME_QUIZ_CREATION row per galgame_quiz, maintained by
-- a trigger and backfilled from existing rows. galgame_id = 0 on purpose — a quiz
-- is not scoped to a single game (it may link several, or be general trivia), so
-- it is never NSFW-dropped by the feed's galgame enrichment. content = the 题干.
-- Idempotent (reuses the shared feed_upsert / feed_delete helpers).

-- NOTE galgame_quiz.id is BIGINT (unlike the other feed sources' INT ids), so
-- NEW.id / OLD.id must be cast to integer or the feed_upsert/feed_delete calls
-- (INT source_id params) fail to resolve ("function does not exist").
CREATE OR REPLACE FUNCTION feed_sync_galgame_quiz() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_QUIZ_CREATION', OLD.id::integer); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_QUIZ_CREATION', NEW.id::integer, NEW.user_id, 0, NEW.question, '/galgame-quiz/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_feed_galgame_quiz ON galgame_quiz;
CREATE TRIGGER trg_feed_galgame_quiz AFTER INSERT OR UPDATE OR DELETE ON galgame_quiz FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_quiz();

-- Backfill existing quizzes.
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_QUIZ_CREATION', q.id, q.user_id, 0, q.question, '/galgame-quiz/' || q.id, false, q.created FROM galgame_quiz q
ON CONFLICT (type, source_id) DO NOTHING;
