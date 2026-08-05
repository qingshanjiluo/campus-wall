-- 035: NSFW filtering for topic + galgame activity in the materialized feed.
--
-- 034 hardcoded is_nsfw=false for every source except websites, so:
--   • NSFW topics (topic.is_nsfw) AND their replies/comments/upvotes — plus the
--     推话题/最佳答案 messages that point at them — leaked into the SFW feed.
--   • galgame_rating_comment carried galgame_id=0, so it dodged the enrichment's
--     NSFW-galgame drop (every other galgame-scoped row carries galgame_id and is
--     filtered there; galgame NSFW is wiki-owned, not a local column).
--
-- This migration:
--   • topics: feed is_nsfw = topic.is_nsfw; replies/comments/upvotes + the linked
--     messages inherit the parent topic's flag; a topic is_nsfw FLIP cascades to
--     all of them.
--   • galgame_rating_comment: carry the rating's galgame_id so enrichment drops it
--     for NSFW galgames like every other galgame-scoped row.
--
-- CREATE OR REPLACE keeps the triggers attached in 034 bound to the new bodies, so
-- no trigger re-attach is needed. Idempotent (the backfill guards on IS DISTINCT).
BEGIN;

-- ── topics: is_nsfw from topic.is_nsfw + cascade on a flip ──────────────────────
CREATE OR REPLACE FUNCTION feed_sync_topic() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_CREATION', OLD.id); RETURN OLD; END IF;
    IF NEW.status = 1 THEN PERFORM feed_delete('TOPIC_CREATION', NEW.id); RETURN NEW; END IF;
    PERFORM feed_upsert('TOPIC_CREATION', NEW.id, NEW.user_id, 0, NEW.title, '/topic/' || NEW.id, NEW.is_nsfw, NEW.created);
    -- A change to the topic's NSFW flag re-flags everything that hangs off it.
    IF TG_OP = 'UPDATE' AND NEW.is_nsfw IS DISTINCT FROM OLD.is_nsfw THEN
        UPDATE feed_activity fa SET is_nsfw = NEW.is_nsfw
          FROM topic_reply r   WHERE fa.type = 'TOPIC_REPLY_CREATION'   AND fa.source_id = r.id AND r.topic_id = NEW.id;
        UPDATE feed_activity fa SET is_nsfw = NEW.is_nsfw
          FROM topic_comment c WHERE fa.type = 'TOPIC_COMMENT_CREATION' AND fa.source_id = c.id AND c.topic_id = NEW.id;
        UPDATE feed_activity fa SET is_nsfw = NEW.is_nsfw
          FROM topic_upvote u  WHERE fa.type = 'TOPIC_UPVOTE'           AND fa.source_id = u.id AND u.topic_id = NEW.id;
        UPDATE feed_activity fa SET is_nsfw = NEW.is_nsfw
          WHERE fa.type IN ('MESSAGE_UPVOTE', 'MESSAGE_SOLUTION') AND fa.link = '/topic/' || NEW.id;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_reply() RETURNS trigger AS $$
DECLARE v_nsfw boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_REPLY_CREATION', OLD.id); RETURN OLD; END IF;
    SELECT is_nsfw INTO v_nsfw FROM topic WHERE id = NEW.topic_id;
    PERFORM feed_upsert('TOPIC_REPLY_CREATION', NEW.id, NEW.user_id, 0, COALESCE(NEW.content, ''), '/topic/' || NEW.topic_id, COALESCE(v_nsfw, false), NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_comment() RETURNS trigger AS $$
DECLARE v_nsfw boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    SELECT is_nsfw INTO v_nsfw FROM topic WHERE id = NEW.topic_id;
    PERFORM feed_upsert('TOPIC_COMMENT_CREATION', NEW.id, NEW.user_id, 0, SUBSTRING(NEW.content, 1, 100), '/topic/' || NEW.topic_id, COALESCE(v_nsfw, false), NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_upvote() RETURNS trigger AS $$
DECLARE v_nsfw boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_UPVOTE', OLD.id); RETURN OLD; END IF;
    SELECT is_nsfw INTO v_nsfw FROM topic WHERE id = NEW.topic_id;
    PERFORM feed_upsert('TOPIC_UPVOTE', NEW.id, NEW.user_id, 0, COALESCE(NEW.description, ''), '/topic/' || NEW.topic_id, COALESCE(v_nsfw, false), NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- ── messages (推话题 / 最佳答案): inherit the linked topic's NSFW flag ──────────
CREATE OR REPLACE FUNCTION feed_sync_message() RETURNS trigger AS $$
DECLARE v_type text; v_tid int; v_nsfw boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM feed_delete('MESSAGE_UPVOTE', OLD.id);
        PERFORM feed_delete('MESSAGE_SOLUTION', OLD.id);
        RETURN OLD;
    END IF;
    v_type := CASE NEW.type WHEN 'upvoted' THEN 'MESSAGE_UPVOTE' WHEN 'solution' THEN 'MESSAGE_SOLUTION' ELSE NULL END;
    IF v_type IS NOT NULL THEN
        v_tid := (regexp_match(NEW.link, '^/topic/([0-9]+)'))[1]::int;
        IF v_tid IS NOT NULL THEN SELECT is_nsfw INTO v_nsfw FROM topic WHERE id = v_tid; END IF;
        PERFORM feed_upsert(v_type, NEW.id, NEW.sender_id, 0, NEW.content, NEW.link, COALESCE(v_nsfw, false), NEW.created);
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- ── galgame_rating_comment: carry the rating's galgame_id (enrichment drops NSFW) ─
CREATE OR REPLACE FUNCTION feed_sync_galgame_rating_comment() RETURNS trigger AS $$
DECLARE v_gid int;
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_RATING_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    SELECT galgame_id INTO v_gid FROM galgame_rating WHERE id = NEW.galgame_rating_id;
    PERFORM feed_upsert('GALGAME_RATING_COMMENT_CREATION', NEW.id, NEW.user_id, COALESCE(v_gid, 0), SUBSTRING(NEW.content, 1, 100), '/galgame-rating/' || NEW.galgame_rating_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- ── backfill existing feed rows ─────────────────────────────────────────────────
UPDATE feed_activity fa SET is_nsfw = t.is_nsfw
  FROM topic t
  WHERE fa.type = 'TOPIC_CREATION' AND fa.source_id = t.id AND fa.is_nsfw IS DISTINCT FROM t.is_nsfw;

UPDATE feed_activity fa SET is_nsfw = t.is_nsfw
  FROM topic_reply r JOIN topic t ON t.id = r.topic_id
  WHERE fa.type = 'TOPIC_REPLY_CREATION' AND fa.source_id = r.id AND fa.is_nsfw IS DISTINCT FROM t.is_nsfw;

UPDATE feed_activity fa SET is_nsfw = t.is_nsfw
  FROM topic_comment c JOIN topic t ON t.id = c.topic_id
  WHERE fa.type = 'TOPIC_COMMENT_CREATION' AND fa.source_id = c.id AND fa.is_nsfw IS DISTINCT FROM t.is_nsfw;

UPDATE feed_activity fa SET is_nsfw = t.is_nsfw
  FROM topic_upvote u JOIN topic t ON t.id = u.topic_id
  WHERE fa.type = 'TOPIC_UPVOTE' AND fa.source_id = u.id AND fa.is_nsfw IS DISTINCT FROM t.is_nsfw;

UPDATE feed_activity fa SET is_nsfw = t.is_nsfw
  FROM topic t
  WHERE fa.type IN ('MESSAGE_UPVOTE', 'MESSAGE_SOLUTION')
    AND t.id = (regexp_match(fa.link, '^/topic/([0-9]+)'))[1]::int
    AND fa.is_nsfw IS DISTINCT FROM t.is_nsfw;

UPDATE feed_activity fa SET galgame_id = gr.galgame_id
  FROM galgame_rating_comment rc JOIN galgame_rating gr ON gr.id = rc.galgame_rating_id
  WHERE fa.type = 'GALGAME_RATING_COMMENT_CREATION' AND fa.source_id = rc.id AND fa.galgame_id IS DISTINCT FROM gr.galgame_id;

COMMIT;
