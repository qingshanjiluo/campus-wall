-- 046: add spoiler_level to galgame_quiz (none/portion/serious), mirroring the
-- galgame_rating spoiler scale — some quiz questions reveal plot. Separate
-- migration (not folded into 045) so it applies cleanly whether or not 045 has
-- already run. Idempotent: the inline CHECK is only added together with the
-- column (ADD COLUMN IF NOT EXISTS skips both on re-run).
BEGIN;

ALTER TABLE galgame_quiz
    ADD COLUMN IF NOT EXISTS spoiler_level VARCHAR(16) NOT NULL DEFAULT 'none'
    CHECK (spoiler_level IN ('none', 'portion', 'serious'));

COMMIT;
