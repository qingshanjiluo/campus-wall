-- 050: windowed view stats for galgame / topic / galgame-quiz. Each entity keeps
-- its total `view`; a per-domain daily bucket table records per-day views, and a
-- daily rollup (cmd/view-rollup) sums them onto view_7d / view_30d so list pages
-- can sort by "本周/本月热度" cheaply. No FK on entity_id (uniform BIGINT across
-- domains; orphans are pruned by the rollup). Idempotent.
BEGIN;

ALTER TABLE galgame ADD COLUMN IF NOT EXISTS view_7d INT NOT NULL DEFAULT 0;
ALTER TABLE galgame ADD COLUMN IF NOT EXISTS view_30d INT NOT NULL DEFAULT 0;
ALTER TABLE topic ADD COLUMN IF NOT EXISTS view_7d INT NOT NULL DEFAULT 0;
ALTER TABLE topic ADD COLUMN IF NOT EXISTS view_30d INT NOT NULL DEFAULT 0;
ALTER TABLE galgame_quiz ADD COLUMN IF NOT EXISTS view_7d INT NOT NULL DEFAULT 0;
ALTER TABLE galgame_quiz ADD COLUMN IF NOT EXISTS view_30d INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS galgame_view_daily (
    entity_id BIGINT NOT NULL,
    day       DATE   NOT NULL,
    count     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_id, day)
);
CREATE INDEX IF NOT EXISTS idx_galgame_view_daily_day ON galgame_view_daily (day);

CREATE TABLE IF NOT EXISTS topic_view_daily (
    entity_id BIGINT NOT NULL,
    day       DATE   NOT NULL,
    count     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_id, day)
);
CREATE INDEX IF NOT EXISTS idx_topic_view_daily_day ON topic_view_daily (day);

CREATE TABLE IF NOT EXISTS galgame_quiz_view_daily (
    entity_id BIGINT NOT NULL,
    day       DATE   NOT NULL,
    count     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (entity_id, day)
);
CREATE INDEX IF NOT EXISTS idx_galgame_quiz_view_daily_day ON galgame_quiz_view_daily (day);

COMMIT;
