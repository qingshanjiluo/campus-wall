-- Best-effort rollback (mirrors the 049 precedent): recreate the five tables as
-- EMPTY structures. The migrated comment DATA is NOT restored (it lives in the
-- community primitive now), and the legacy feed trigger functions/triggers are
-- NOT recreated — this down is a structural placeholder only, effectively
-- irreversible for content.
CREATE TABLE IF NOT EXISTS galgame_comment (
    id                SERIAL PRIMARY KEY,
    content           VARCHAR(5000) NOT NULL,
    galgame_id        INTEGER       NOT NULL,
    user_id           INTEGER       NOT NULL,
    parent_comment_id INTEGER,
    root_comment_id   INTEGER,
    like_count        INTEGER       NOT NULL DEFAULT 0,
    status            INTEGER       NOT NULL DEFAULT 0,
    edited            TIMESTAMPTZ,
    created           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated           TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS galgame_comment_like (
    id                 SERIAL PRIMARY KEY,
    user_id            INTEGER     NOT NULL,
    galgame_comment_id INTEGER     NOT NULL,
    created            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS galgame_rating_comment (
    id                SERIAL PRIMARY KEY,
    content           VARCHAR(1314) NOT NULL DEFAULT '',
    galgame_rating_id INTEGER       NOT NULL,
    user_id           INTEGER       NOT NULL,
    target_user_id    INTEGER,
    created           TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated           TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS galgame_website_comment (
    id         SERIAL PRIMARY KEY,
    content    TEXT        NOT NULL DEFAULT '',
    edited     TIMESTAMPTZ,
    user_id    INTEGER     NOT NULL,
    website_id INTEGER     NOT NULL,
    parent_id  INTEGER,
    created    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS galgame_toolset_comment (
    id         SERIAL PRIMARY KEY,
    content    TEXT        NOT NULL DEFAULT '',
    edited     TIMESTAMPTZ,
    user_id    INTEGER     NOT NULL,
    toolset_id INTEGER     NOT NULL,
    parent_id  INTEGER,
    created    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated    TIMESTAMPTZ NOT NULL DEFAULT now()
);
