DROP TRIGGER IF EXISTS trg_feed_galgame_quiz ON galgame_quiz;
DROP FUNCTION IF EXISTS feed_sync_galgame_quiz();
DELETE FROM feed_activity WHERE type = 'GALGAME_QUIZ_CREATION';
