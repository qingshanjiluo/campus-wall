-- Revert the feed trigger functions to their pre-055 form (no status guard).
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

CREATE OR REPLACE FUNCTION feed_sync_galgame_comment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_COMMENT_CREATION', NEW.id, NEW.user_id, NEW.galgame_id, NEW.content, '/galgame/' || NEW.galgame_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;
