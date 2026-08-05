-- 029: add topic.cover_images — an optional ordered list of 1..9 cover images
-- shown on the home/feed topic card.
--
-- STORAGE = a JSON array of /image/<hash> CONTENT TOKENS in a plain text column,
-- NOT a text[] of bare hashes and NOT jsonb. This is deliberate:
--
--   The daily reference-ping cron (internal/infrastructure/cron/reference_ping.go)
--   keeps every forum-referenced image alive by enumerating EVERY text/varchar/char
--   column from information_schema and scanning it for /image/<hash> tokens. Array
--   and jsonb columns have data_type 'ARRAY'/'jsonb' and are invisible to that
--   schema-derived scan; a bare 64-hex hash also never matches its /image/<hash>
--   regex. Storing the covers as /image/<hash> tokens in a scalar text column means
--   topic covers are kept alive for free — the ping needs no column-list edit and
--   cannot drift out of sync (the exact failure the schema-derived design prevents).
--
-- Empty string '' = no covers (the natural default for every existing row); the
-- ping's `LIKE '%/image/%'` pre-filter skips those rows at no cost. Idempotent.
BEGIN;

ALTER TABLE public.topic
  ADD COLUMN IF NOT EXISTS cover_images text NOT NULL DEFAULT '';

COMMENT ON COLUMN public.topic.cover_images IS
  'Optional 1..9 cover images for the feed card, stored as a JSON array of /image/<hash> content tokens (see migration 029 + cron/reference_ping.go). '''' = none.';

COMMIT;
