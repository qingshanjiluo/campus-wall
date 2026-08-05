-- Reverse of 065: drop the galgame-resource / quiz community-comment display
-- counters. The comments themselves live in the community primitive and are
-- untouched by this.
ALTER TABLE galgame_resource DROP COLUMN IF EXISTS comment_count;
ALTER TABLE galgame_quiz DROP COLUMN IF EXISTS comment_count;
