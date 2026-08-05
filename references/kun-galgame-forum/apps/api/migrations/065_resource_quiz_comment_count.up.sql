-- galgame_resource.comment_count / galgame_quiz.comment_count: LIVE display
-- counters for the two comment areas introduced directly on the infra community
-- primitive (the galgame-resource detail page and the quiz play page).
--
-- Unlike the website (migration 034 trigger → 058) and toolset (059) counters,
-- these two areas have NO legacy in-forum comment table and no cutover import
-- behind them: they start empty, so DEFAULT 0 is already correct and there is
-- NOTHING to backfill. The columns are maintained ±1 by the community comment BFF
-- (resource_comment_write.go: create +1 / tombstone -1, via bumpCountColumn),
-- exactly like the website/toolset counters, and are a tolerated display counter
-- (charter ruling 11) — drift never breaks a read.
--
-- Additive + idempotent, so it auto-runs on deploy (NOT in the migrate --exclude
-- list). Safe to apply before or after the code that writes it: an unwritten
-- column just reads 0, and the BFF's counter update is best-effort.
ALTER TABLE galgame_resource ADD COLUMN IF NOT EXISTS comment_count INT NOT NULL DEFAULT 0;
ALTER TABLE galgame_quiz ADD COLUMN IF NOT EXISTS comment_count INT NOT NULL DEFAULT 0;
