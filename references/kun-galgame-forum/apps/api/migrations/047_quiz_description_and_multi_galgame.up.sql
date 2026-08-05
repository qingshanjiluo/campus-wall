-- 047: quiz gains a markdown `description` (intro / image hints), a
-- `hide_galgame` flag (linked games revealed only after the viewer answers),
-- and MULTIPLE galgame associations via a join table (replacing the single
-- galgame_id, which is kept for one release then dropped — deploy-then-drop).
-- Idempotent.
BEGIN;

ALTER TABLE galgame_quiz
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE galgame_quiz
    ADD COLUMN IF NOT EXISTS hide_galgame BOOLEAN NOT NULL DEFAULT false;

-- Many-to-many: a quiz can reference several galgames.
CREATE TABLE IF NOT EXISTS galgame_quiz_galgame (
    quiz_id    BIGINT  NOT NULL REFERENCES galgame_quiz (id) ON DELETE CASCADE,
    galgame_id INTEGER NOT NULL,
    PRIMARY KEY (quiz_id, galgame_id)
);
-- "quizzes linked to galgame X" (the galgame-detail 题库 tab filter).
CREATE INDEX IF NOT EXISTS idx_gqg_galgame ON galgame_quiz_galgame (galgame_id);

-- Backfill the existing single link into the join table.
INSERT INTO galgame_quiz_galgame (quiz_id, galgame_id)
    SELECT id, galgame_id FROM galgame_quiz WHERE galgame_id IS NOT NULL
    ON CONFLICT DO NOTHING;

COMMIT;
