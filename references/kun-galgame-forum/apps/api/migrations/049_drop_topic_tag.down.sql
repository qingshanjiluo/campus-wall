-- Best-effort rollback: recreate the (empty) tables + column. Tag data is not
-- restored.
CREATE TABLE IF NOT EXISTS topic_tag (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR     NOT NULL UNIQUE,
    created TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS topic_tag_relation (
    topic_id INTEGER     NOT NULL,
    tag_id   INTEGER     NOT NULL REFERENCES topic_tag (id) ON DELETE CASCADE,
    created  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (topic_id, tag_id)
);
ALTER TABLE topic_draft ADD COLUMN IF NOT EXISTS tags TEXT NOT NULL DEFAULT '';
