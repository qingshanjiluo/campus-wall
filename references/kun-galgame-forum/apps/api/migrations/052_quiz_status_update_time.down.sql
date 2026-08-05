DROP INDEX IF EXISTS idx_galgame_quiz_status_update_time;
ALTER TABLE galgame_quiz DROP COLUMN IF EXISTS status_update_time;
