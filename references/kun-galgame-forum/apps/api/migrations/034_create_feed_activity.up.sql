-- Materialized merged activity feed. Replaces the read-time UNION across ~18
-- source tables with ONE keyset-paginated table: every source writes a row here
-- via triggers (the same projection the old Sources map computed), so the feed
-- reads a single indexed table — flat cost per page regardless of depth / source
-- count, and a custom tab becomes `WHERE type = ANY(...)`. Enrichment is
-- unchanged (it still gets the ActivityRow shape: type/source_id/content/link/…).
-- Idempotent.

CREATE TABLE IF NOT EXISTS feed_activity (
    id         BIGSERIAL PRIMARY KEY,
    type       VARCHAR(64) NOT NULL,
    source_id  INTEGER     NOT NULL,
    user_id    INTEGER     NOT NULL DEFAULT 0,
    galgame_id INTEGER     NOT NULL DEFAULT 0,
    content    TEXT        NOT NULL DEFAULT '',
    link       TEXT        NOT NULL DEFAULT '',
    is_nsfw    BOOLEAN     NOT NULL DEFAULT false,
    created    TIMESTAMPTZ NOT NULL,
    UNIQUE (type, source_id)
);
-- Keyset indexes. The total order is (created, type, source_id) DESC — matching
-- the existing activity cursor — so the first index serves the timeline + common
-- tabs, and the type-leading one serves rare-type tabs (seek by type, then created).
CREATE INDEX IF NOT EXISTS idx_feed_activity_keyset ON feed_activity (created DESC, type DESC, source_id DESC);
CREATE INDEX IF NOT EXISTS idx_feed_activity_type_keyset ON feed_activity (type, created DESC, source_id DESC);

-- ── upsert / delete helpers (one INSERT … ON CONFLICT, reused by every trigger) ──
CREATE OR REPLACE FUNCTION feed_upsert(
    p_type text, p_sid int, p_uid int, p_gid int,
    p_content text, p_link text, p_nsfw boolean, p_created timestamptz
) RETURNS void AS $$
    INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
    VALUES (p_type, p_sid, p_uid, p_gid, p_content, p_link, p_nsfw, p_created)
    ON CONFLICT (type, source_id) DO UPDATE SET
        user_id = EXCLUDED.user_id, galgame_id = EXCLUDED.galgame_id,
        content = EXCLUDED.content, link = EXCLUDED.link,
        is_nsfw = EXCLUDED.is_nsfw, created = EXCLUDED.created;
$$ LANGUAGE sql;

CREATE OR REPLACE FUNCTION feed_delete(p_type text, p_sid int) RETURNS void AS $$
    DELETE FROM feed_activity WHERE type = p_type AND source_id = p_sid;
$$ LANGUAGE sql;

-- ── per-source trigger functions ──────────────────────────────────────────────

-- TOPIC_CREATION (status = 1 means hidden → keep it out / pull it).
CREATE OR REPLACE FUNCTION feed_sync_topic() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_CREATION', OLD.id); RETURN OLD; END IF;
    IF NEW.status = 1 THEN PERFORM feed_delete('TOPIC_CREATION', NEW.id); RETURN NEW; END IF;
    PERFORM feed_upsert('TOPIC_CREATION', NEW.id, NEW.user_id, 0, NEW.title, '/topic/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_reply() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_REPLY_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TOPIC_REPLY_CREATION', NEW.id, NEW.user_id, 0, COALESCE(NEW.content, ''), '/topic/' || NEW.topic_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_comment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TOPIC_COMMENT_CREATION', NEW.id, NEW.user_id, 0, SUBSTRING(NEW.content, 1, 100), '/topic/' || NEW.topic_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_topic_upvote() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOPIC_UPVOTE', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TOPIC_UPVOTE', NEW.id, NEW.user_id, 0, COALESCE(NEW.description, ''), '/topic/' || NEW.topic_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- GALGAME_CREATION: local galgame row; actor (user_id) is filled from the wiki at enrichment → 0.
CREATE OR REPLACE FUNCTION feed_sync_galgame() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_CREATION', NEW.id, 0, NEW.id, '', '/galgame/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_comment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_COMMENT_CREATION', NEW.id, NEW.user_id, NEW.galgame_id, NEW.content, '/galgame/' || NEW.galgame_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_resource() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_RESOURCE_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_RESOURCE_CREATION', NEW.id, NEW.user_id, NEW.galgame_id, '', '/galgame/' || NEW.galgame_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- galgame_activity carries its own type (GALGAME_EDIT / GALGAME_PR_CREATION).
CREATE OR REPLACE FUNCTION feed_sync_galgame_activity() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete(OLD.type, OLD.id); RETURN OLD; END IF;
    IF NEW.type IN ('GALGAME_EDIT', 'GALGAME_PR_CREATION') THEN
        PERFORM feed_upsert(NEW.type, NEW.id, NEW.user_id, NEW.galgame_id, '', '/galgame/' || NEW.galgame_id, false, NEW.created);
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_rating() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_RATING_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_RATING_CREATION', NEW.id, NEW.user_id, NEW.galgame_id,
        CASE WHEN NEW.spoiler_level <> 'none' THEN '⚠️ 该评分可能含有剧透内容，点进查看'
             ELSE SUBSTRING(COALESCE(NEW.short_summary, ''), 1, 100) END,
        '/galgame-rating/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_rating_comment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_RATING_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_RATING_COMMENT_CREATION', NEW.id, NEW.user_id, 0, SUBSTRING(NEW.content, 1, 100), '/galgame-rating/' || NEW.galgame_rating_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- Website: is_nsfw from age_limit (SFW viewers hide r18 sites).
CREATE OR REPLACE FUNCTION feed_sync_galgame_website() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_WEBSITE_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('GALGAME_WEBSITE_CREATION', NEW.id, NEW.user_id, 0, NEW.name, '/website/' || NEW.url, (NEW.age_limit <> 'all'), NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- Website comment: link + is_nsfw resolved from the parent website.
CREATE OR REPLACE FUNCTION feed_sync_galgame_website_comment() RETURNS trigger AS $$
DECLARE v_url text; v_nsfw boolean;
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('GALGAME_WEBSITE_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    SELECT w.url, (w.age_limit <> 'all') INTO v_url, v_nsfw FROM galgame_website w WHERE w.id = NEW.website_id;
    PERFORM feed_upsert('GALGAME_WEBSITE_COMMENT_CREATION', NEW.id, NEW.user_id, 0, SUBSTRING(NEW.content, 1, 100), '/website/' || COALESCE(v_url, ''), COALESCE(v_nsfw, false), NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_toolset() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOOLSET_CREATION', OLD.id); RETURN OLD; END IF;
    IF NEW.status = 1 THEN PERFORM feed_delete('TOOLSET_CREATION', NEW.id); RETURN NEW; END IF;
    PERFORM feed_upsert('TOOLSET_CREATION', NEW.id, NEW.user_id, 0, NEW.name, '/toolset/' || NEW.id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_toolset_resource() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOOLSET_RESOURCE_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TOOLSET_RESOURCE_CREATION', NEW.id, NEW.user_id, 0, COALESCE(NULLIF(NEW.note, ''), NEW.content), '/toolset/' || NEW.toolset_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_galgame_toolset_comment() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TOOLSET_COMMENT_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TOOLSET_COMMENT_CREATION', NEW.id, NEW.user_id, 0, SUBSTRING(NEW.content, 1, 100), '/toolset/' || NEW.toolset_id, false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_todo() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('TODO_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('TODO_CREATION', NEW.id, NEW.user_id, 0, NEW.content_zh_cn, '/update', false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION feed_sync_update_log() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN PERFORM feed_delete('UPDATE_LOG_CREATION', OLD.id); RETURN OLD; END IF;
    PERFORM feed_upsert('UPDATE_LOG_CREATION', NEW.id, NEW.user_id, 0, NEW.content_zh_cn, '/update', false, NEW.created);
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- message → MESSAGE_UPVOTE / MESSAGE_SOLUTION (only those two types feed the timeline).
CREATE OR REPLACE FUNCTION feed_sync_message() RETURNS trigger AS $$
DECLARE v_type text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM feed_delete('MESSAGE_UPVOTE', OLD.id);
        PERFORM feed_delete('MESSAGE_SOLUTION', OLD.id);
        RETURN OLD;
    END IF;
    v_type := CASE NEW.type WHEN 'upvoted' THEN 'MESSAGE_UPVOTE' WHEN 'solution' THEN 'MESSAGE_SOLUTION' ELSE NULL END;
    IF v_type IS NOT NULL THEN
        PERFORM feed_upsert(v_type, NEW.id, NEW.sender_id, 0, NEW.content, NEW.link, false, NEW.created);
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

-- ── attach triggers (idempotent: drop then create) ────────────────────────────
DROP TRIGGER IF EXISTS trg_feed_topic ON topic;
CREATE TRIGGER trg_feed_topic AFTER INSERT OR UPDATE OR DELETE ON topic FOR EACH ROW EXECUTE FUNCTION feed_sync_topic();
DROP TRIGGER IF EXISTS trg_feed_topic_reply ON topic_reply;
CREATE TRIGGER trg_feed_topic_reply AFTER INSERT OR UPDATE OR DELETE ON topic_reply FOR EACH ROW EXECUTE FUNCTION feed_sync_topic_reply();
DROP TRIGGER IF EXISTS trg_feed_topic_comment ON topic_comment;
CREATE TRIGGER trg_feed_topic_comment AFTER INSERT OR UPDATE OR DELETE ON topic_comment FOR EACH ROW EXECUTE FUNCTION feed_sync_topic_comment();
DROP TRIGGER IF EXISTS trg_feed_topic_upvote ON topic_upvote;
CREATE TRIGGER trg_feed_topic_upvote AFTER INSERT OR UPDATE OR DELETE ON topic_upvote FOR EACH ROW EXECUTE FUNCTION feed_sync_topic_upvote();
DROP TRIGGER IF EXISTS trg_feed_galgame ON galgame;
CREATE TRIGGER trg_feed_galgame AFTER INSERT OR UPDATE OR DELETE ON galgame FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame();
DROP TRIGGER IF EXISTS trg_feed_galgame_comment ON galgame_comment;
CREATE TRIGGER trg_feed_galgame_comment AFTER INSERT OR UPDATE OR DELETE ON galgame_comment FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_comment();
DROP TRIGGER IF EXISTS trg_feed_galgame_resource ON galgame_resource;
CREATE TRIGGER trg_feed_galgame_resource AFTER INSERT OR UPDATE OR DELETE ON galgame_resource FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_resource();
DROP TRIGGER IF EXISTS trg_feed_galgame_activity ON galgame_activity;
CREATE TRIGGER trg_feed_galgame_activity AFTER INSERT OR UPDATE OR DELETE ON galgame_activity FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_activity();
DROP TRIGGER IF EXISTS trg_feed_galgame_rating ON galgame_rating;
CREATE TRIGGER trg_feed_galgame_rating AFTER INSERT OR UPDATE OR DELETE ON galgame_rating FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_rating();
DROP TRIGGER IF EXISTS trg_feed_galgame_rating_comment ON galgame_rating_comment;
CREATE TRIGGER trg_feed_galgame_rating_comment AFTER INSERT OR UPDATE OR DELETE ON galgame_rating_comment FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_rating_comment();
DROP TRIGGER IF EXISTS trg_feed_galgame_website ON galgame_website;
CREATE TRIGGER trg_feed_galgame_website AFTER INSERT OR UPDATE OR DELETE ON galgame_website FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_website();
DROP TRIGGER IF EXISTS trg_feed_galgame_website_comment ON galgame_website_comment;
CREATE TRIGGER trg_feed_galgame_website_comment AFTER INSERT OR UPDATE OR DELETE ON galgame_website_comment FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_website_comment();
DROP TRIGGER IF EXISTS trg_feed_galgame_toolset ON galgame_toolset;
CREATE TRIGGER trg_feed_galgame_toolset AFTER INSERT OR UPDATE OR DELETE ON galgame_toolset FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_toolset();
DROP TRIGGER IF EXISTS trg_feed_galgame_toolset_resource ON galgame_toolset_resource;
CREATE TRIGGER trg_feed_galgame_toolset_resource AFTER INSERT OR UPDATE OR DELETE ON galgame_toolset_resource FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_toolset_resource();
DROP TRIGGER IF EXISTS trg_feed_galgame_toolset_comment ON galgame_toolset_comment;
CREATE TRIGGER trg_feed_galgame_toolset_comment AFTER INSERT OR UPDATE OR DELETE ON galgame_toolset_comment FOR EACH ROW EXECUTE FUNCTION feed_sync_galgame_toolset_comment();
DROP TRIGGER IF EXISTS trg_feed_todo ON todo;
CREATE TRIGGER trg_feed_todo AFTER INSERT OR UPDATE OR DELETE ON todo FOR EACH ROW EXECUTE FUNCTION feed_sync_todo();
DROP TRIGGER IF EXISTS trg_feed_update_log ON update_log;
CREATE TRIGGER trg_feed_update_log AFTER INSERT OR UPDATE OR DELETE ON update_log FOR EACH ROW EXECUTE FUNCTION feed_sync_update_log();
DROP TRIGGER IF EXISTS trg_feed_message ON message;
CREATE TRIGGER trg_feed_message AFTER INSERT OR UPDATE OR DELETE ON message FOR EACH ROW EXECUTE FUNCTION feed_sync_message();

-- ── backfill from existing rows (mirrors the old Sources projections) ─────────
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOPIC_CREATION', t.id, t.user_id, 0, t.title, '/topic/' || t.id, false, t.created FROM topic t WHERE t.status <> 1
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOPIC_REPLY_CREATION', t.id, t.user_id, 0, COALESCE(t.content, ''), '/topic/' || t.topic_id, false, t.created FROM topic_reply t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOPIC_COMMENT_CREATION', t.id, t.user_id, 0, SUBSTRING(t.content, 1, 100), '/topic/' || t.topic_id, false, t.created FROM topic_comment t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOPIC_UPVOTE', t.id, t.user_id, 0, COALESCE(t.description, ''), '/topic/' || t.topic_id, false, t.created FROM topic_upvote t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_CREATION', t.id, 0, t.id, '', '/galgame/' || t.id, false, t.created FROM galgame t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_COMMENT_CREATION', t.id, t.user_id, t.galgame_id, t.content, '/galgame/' || t.galgame_id, false, t.created FROM galgame_comment t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_RESOURCE_CREATION', t.id, t.user_id, t.galgame_id, '', '/galgame/' || t.galgame_id, false, t.created FROM galgame_resource t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT t.type, t.id, t.user_id, t.galgame_id, '', '/galgame/' || t.galgame_id, false, t.created FROM galgame_activity t WHERE t.type IN ('GALGAME_EDIT', 'GALGAME_PR_CREATION')
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_RATING_CREATION', t.id, t.user_id, t.galgame_id,
    CASE WHEN t.spoiler_level <> 'none' THEN '⚠️ 该评分可能含有剧透内容，点进查看' ELSE SUBSTRING(COALESCE(t.short_summary, ''), 1, 100) END,
    '/galgame-rating/' || t.id, false, t.created FROM galgame_rating t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_RATING_COMMENT_CREATION', t.id, t.user_id, 0, SUBSTRING(t.content, 1, 100), '/galgame-rating/' || t.galgame_rating_id, false, t.created FROM galgame_rating_comment t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_WEBSITE_CREATION', t.id, t.user_id, 0, t.name, '/website/' || t.url, (t.age_limit <> 'all'), t.created FROM galgame_website t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'GALGAME_WEBSITE_COMMENT_CREATION', t.id, t.user_id, 0, SUBSTRING(t.content, 1, 100),
    '/website/' || COALESCE((SELECT w.url FROM galgame_website w WHERE w.id = t.website_id), ''),
    COALESCE((SELECT w.age_limit <> 'all' FROM galgame_website w WHERE w.id = t.website_id), false), t.created FROM galgame_website_comment t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOOLSET_CREATION', t.id, t.user_id, 0, t.name, '/toolset/' || t.id, false, t.created FROM galgame_toolset t WHERE t.status <> 1
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOOLSET_RESOURCE_CREATION', t.id, t.user_id, 0, COALESCE(NULLIF(t.note, ''), t.content), '/toolset/' || t.toolset_id, false, t.created FROM galgame_toolset_resource t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TOOLSET_COMMENT_CREATION', t.id, t.user_id, 0, SUBSTRING(t.content, 1, 100), '/toolset/' || t.toolset_id, false, t.created FROM galgame_toolset_comment t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'TODO_CREATION', t.id, t.user_id, 0, t.content_zh_cn, '/update', false, t.created FROM todo t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT 'UPDATE_LOG_CREATION', t.id, t.user_id, 0, t.content_zh_cn, '/update', false, t.created FROM update_log t
ON CONFLICT (type, source_id) DO NOTHING;
INSERT INTO feed_activity (type, source_id, user_id, galgame_id, content, link, is_nsfw, created)
SELECT CASE t.type WHEN 'upvoted' THEN 'MESSAGE_UPVOTE' ELSE 'MESSAGE_SOLUTION' END, t.id, t.sender_id, 0, t.content, t.link, false, t.created FROM message t WHERE t.type IN ('upvoted', 'solution')
ON CONFLICT (type, source_id) DO NOTHING;
