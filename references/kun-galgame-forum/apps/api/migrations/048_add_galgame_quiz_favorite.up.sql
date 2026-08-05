-- 048: quizzes can be favorited (收藏). Separate per-domain table (not a shared
-- polymorphic favorites table) — the deliberate best practice. Idempotent.
BEGIN;

ALTER TABLE galgame_quiz
    ADD COLUMN IF NOT EXISTS favorite_count INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS galgame_quiz_favorite (
    quiz_id BIGINT      NOT NULL REFERENCES galgame_quiz (id) ON DELETE CASCADE,
    user_id INTEGER     NOT NULL,
    created TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (quiz_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_gqf_user ON galgame_quiz_favorite (user_id);

COMMIT;
