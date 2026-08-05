-- 063: per-user permission overrides (permission-first authz, Phase 3 / Plan A).
--
-- Where migration 062 stores per-ROLE deltas, this table stores per-USER deltas:
-- one row per (user_id, permission), effect 'grant' adds a key the user's
-- role-derived effective set lacks and 'revoke' removes one it holds. So
-- effective(user) = ((role baseline ± role overrides) ∪ personal grants) −
-- personal revokes. A personal revoke beats any role grant; a personal grant
-- works even for a roleless plain user (the whole point of Plan A). An EMPTY
-- table means every user holds exactly their role-derived set.
--
-- EXCEPTION: a user whose roles include `ren` always holds the full catalog —
-- the apply path pins ren-holders and the write path refuses to store rows
-- targeting them, so overrides can never reduce a ren-holder.
--
-- Additive + idempotent (IF NOT EXISTS), so it auto-runs on deploy — it is NOT
-- in the migrate --exclude list.
CREATE TABLE IF NOT EXISTS user_permission_override (
    user_id    integer NOT NULL,
    permission text NOT NULL,
    effect     text NOT NULL CHECK (effect IN ('grant', 'revoke')),
    updated_by integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, permission)
);
