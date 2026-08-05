-- 064: append-only permission-override audit log (permission-first authz,
-- Phase 3). Every role override replace (migration 062) and every user override
-- replace (migration 063) writes exactly one row here IN THE SAME TRANSACTION as
-- the override change, capturing the before/after delta sets. It is append-only
-- (never updated or deleted by the app) — the tamper-evident trail of who
-- changed which subject's permissions and when.
--
-- subject_kind = 'role' | 'user'; subject = the role name or the target uid as a
-- decimal string. action = 'reset' when the new set is empty (all deltas
-- cleared), else 'replace'. before_rows / after_rows are compact JSON arrays of
-- {"permission","effect"} (no timestamps inside).
--
-- Additive + idempotent (IF NOT EXISTS), so it auto-runs on deploy — it is NOT
-- in the migrate --exclude list.
CREATE TABLE IF NOT EXISTS permission_audit_log (
    id           bigserial PRIMARY KEY,
    operator_id  integer NOT NULL,
    subject_kind text NOT NULL CHECK (subject_kind IN ('role', 'user')),
    subject      text NOT NULL,
    action       text NOT NULL CHECK (action IN ('replace', 'reset')),
    before_rows  jsonb NOT NULL DEFAULT '[]'::jsonb,
    after_rows   jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_permission_audit_log_created ON permission_audit_log (created_at DESC);
