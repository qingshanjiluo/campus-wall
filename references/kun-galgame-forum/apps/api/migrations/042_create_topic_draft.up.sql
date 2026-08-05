-- 042: topic_draft — per-user saved drafts of an unpublished topic. PRIVATE to
-- the author (every query is scoped to user_id in the service, never by the DB).
-- A draft holds the same fields as a create-topic request so it loads straight
-- back into the editor.
--
-- content and cover_images are plain TEXT columns holding /image/<hash> tokens,
-- so the daily reference-ping cron (internal/infrastructure/cron/reference_ping.go)
-- keeps drafted images alive for free — same reason topic.cover_images is text,
-- NOT text[]/jsonb (see migration 029). tags/sections are JSON-array strings in
-- text too (no images, but consistent with the ImageTokens storage pattern).
-- Idempotent.
BEGIN;

CREATE TABLE IF NOT EXISTS topic_draft (
    id           BIGSERIAL    PRIMARY KEY,
    user_id      INTEGER      NOT NULL,
    title        VARCHAR(233) NOT NULL DEFAULT '',
    content      TEXT         NOT NULL DEFAULT '',
    category     VARCHAR(32)  NOT NULL DEFAULT '',
    sections     TEXT         NOT NULL DEFAULT '',
    tags         TEXT         NOT NULL DEFAULT '',
    cover_images TEXT         NOT NULL DEFAULT '',
    is_nsfw      BOOLEAN      NOT NULL DEFAULT false,
    created      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- List a user's drafts, newest first.
CREATE INDEX IF NOT EXISTS idx_topic_draft_user ON topic_draft (user_id, updated DESC);

COMMENT ON COLUMN public.topic_draft.cover_images IS
  'Draft cover images as a JSON array of /image/<hash> tokens in text, kept alive by cron/reference_ping.go (see migration 029). '''' = none.';

COMMIT;
