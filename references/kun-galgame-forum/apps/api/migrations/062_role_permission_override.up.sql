-- 062: runtime role→permission overrides (permission-first authz, Phase 2).
--
-- The compiled baseline (pkg/perm) is the safe known state; this table stores
-- ONLY the DEVIATIONS from it, one row per (role, permission): effect 'grant'
-- adds a key the role's baseline lacks, 'revoke' removes one it holds. So
-- effective(role) = (baseline ∪ grants) − revokes, and an EMPTY table means
-- every role holds exactly its compiled baseline. Resetting a role = deleting
-- its rows.
--
-- Editable roles are exactly {creator, moderator, admin}. `ren` is PINNED to the
-- full permission catalog and is NEVER stored here (the apply path ignores any
-- stray ren row defensively); `user` is implicit in the OAuth contract (never a
-- claim) so it is not manageable either.
--
-- Additive + idempotent (IF NOT EXISTS), so it auto-runs on deploy — it is NOT
-- in the migrate --exclude list.
CREATE TABLE IF NOT EXISTS role_permission_override (
    role       text NOT NULL,
    permission text NOT NULL,
    effect     text NOT NULL CHECK (effect IN ('grant', 'revoke')),
    updated_by integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role, permission)
);
