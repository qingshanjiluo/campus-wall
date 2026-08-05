-- Reverse of 059: drop the toolset community-comment display counter.
ALTER TABLE galgame_toolset DROP COLUMN IF EXISTS comment_count;
