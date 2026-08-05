-- 043: galgame_collection — user-owned galgame collections (收藏夹), replacing the
-- flat galgame_favorite list. A galgame can live in many of a user's collections
-- (many-to-many). Each collection carries a privacy setting: public / private /
-- restricted (visible only to an explicit viewer allow-list).
--
-- Existing galgame_favorite rows are migrated into a per-user default collection
-- ("默认收藏夹", public). galgame_favorite itself is kept for one release as a
-- safety net and dropped later (deploy-then-drop, migration 044).
--
-- galgame.favorite_count keeps its old meaning — the number of DISTINCT users who
-- have the game in >=1 collection — so it needs no recompute here: after the
-- backfill each user still owns exactly one collection per favorited game.
-- Idempotent.
BEGIN;

CREATE TABLE IF NOT EXISTS galgame_collection (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     INTEGER      NOT NULL,                    -- owner, aligns with OAuth users.id
    name        VARCHAR(60)  NOT NULL,
    description VARCHAR(500)  NOT NULL DEFAULT '',
    visibility  VARCHAR(16)  NOT NULL DEFAULT 'public'
                  CHECK (visibility IN ('public', 'private', 'restricted')),
    is_default  BOOLEAN      NOT NULL DEFAULT false,
    item_count  INTEGER      NOT NULL DEFAULT 0,           -- denormalized member count
    created     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- List a user's collections, newest first.
CREATE INDEX IF NOT EXISTS idx_galgame_collection_user ON galgame_collection (user_id, updated DESC);
-- At most one default collection per user (the migrated "默认收藏夹").
CREATE UNIQUE INDEX IF NOT EXISTS ux_galgame_collection_default ON galgame_collection (user_id) WHERE is_default;

CREATE TABLE IF NOT EXISTS galgame_collection_item (
    id            BIGSERIAL   PRIMARY KEY,
    collection_id BIGINT      NOT NULL REFERENCES galgame_collection (id) ON DELETE CASCADE,
    galgame_id    INTEGER     NOT NULL,
    user_id       INTEGER     NOT NULL,                    -- denormalized owner (index-only hot reads)
    created       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (collection_id, galgame_id)
);

-- "Is this game in any of my collections" + distinct-user favorite_count, both index-only.
CREATE INDEX IF NOT EXISTS idx_gci_user_galgame ON galgame_collection_item (user_id, galgame_id);
-- Reverse lookup / per-game counting.
CREATE INDEX IF NOT EXISTS idx_gci_galgame ON galgame_collection_item (galgame_id);

CREATE TABLE IF NOT EXISTS galgame_collection_viewer (
    id            BIGSERIAL   PRIMARY KEY,
    collection_id BIGINT      NOT NULL REFERENCES galgame_collection (id) ON DELETE CASCADE,
    user_id       INTEGER     NOT NULL,                    -- authorized viewer, aligns with OAuth users.id
    created       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (collection_id, user_id)
);

COMMENT ON COLUMN public.galgame_collection.visibility IS
  'public = anyone; private = owner only; restricted = owner + galgame_collection_viewer rows.';

-- ── Backfill: every existing favorite → the user's default collection ──
-- The default collection stores an EMPTY name AND description: identity (the
-- username) is OAuth-owned and absent from this DB (contract C6), so the UI
-- renders them dynamically for is_default rows (see app/utils/collection.ts):
--   name        → "<用户名>的收藏夹"
--   description → "<用户名>所有收藏的游戏已经被放置在 <用户名>的收藏夹"
INSERT INTO galgame_collection (user_id, name, description, visibility, is_default, created, updated)
SELECT DISTINCT user_id,
       '',
       '',
       'public',
       true,
       now(),
       now()
FROM galgame_favorite
ON CONFLICT DO NOTHING;                                    -- idempotent via ux_galgame_collection_default

INSERT INTO galgame_collection_item (collection_id, galgame_id, user_id, created, updated)
SELECT c.id, f.galgame_id, f.user_id, f.created, now()
FROM galgame_favorite f
JOIN galgame_collection c ON c.user_id = f.user_id AND c.is_default
ON CONFLICT (collection_id, galgame_id) DO NOTHING;

UPDATE galgame_collection c
SET item_count = (SELECT count(*) FROM galgame_collection_item i WHERE i.collection_id = c.id);

COMMIT;
