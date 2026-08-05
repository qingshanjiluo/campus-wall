-- 012: Per-user read-state for system_message (admin broadcasts).
--
-- Replaces the row-level `system_message.status` field, which was a global
-- footgun: any single user clicking "mark all read" flipped the badge for
-- EVERY other user (see the removed TODO in handler.MarkAdminRead).
--
-- Modeled after migration 008 (wiki_message_read_state) — same
-- high-water-mark cursor pattern:
--   "user X has read every system_message whose id <= last_read_message_id"
-- Mark-all-read just bumps the cursor to MAX(id); unread count is the
-- number of rows above the cursor. No fan-out write needed.
--
-- RUN ORDER: this migration is in cmd/migrate's default --exclude list
-- and must be invoked explicitly via `--only=012` AFTER OAuth-side
-- migrate-users has completed. Reason: step 2 below backfills cursors
-- keyed on "user".id, and migrate-users rewrites "user".id (plus a
-- hard-coded list of dependent user_id columns) to align with OAuth's
-- ID space. If 012 ran first, its cursor rows would point at pre-remap
-- IDs that no longer match any "user".id, silently nullifying the
-- "everything pre-migration is already read" intent. Recommended
-- position in the runbook: immediately after `--only=007`.

BEGIN;

-- ──────────────────────────────────────────
-- 1. New per-user cursor table.
-- ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS system_message_read_state (
    user_id              INT PRIMARY KEY,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ──────────────────────────────────────────
-- 2. Backfill: seed cursor = MAX(system_message.id) for every existing user.
--
-- The old row-level `status` had no per-user truth to preserve, so we treat
-- the migration moment as a clean break: "everything currently in the table
-- is already read for everyone." The alternative (cursor=0) would surprise
-- every active user with a backlog of red dots they never personally left
-- unread — strictly worse UX for zero correctness gain.
--
-- New messages posted AFTER this migration naturally appear as unread
-- (id > cursor). New users who sign up later will get cursor=0 on first
-- read-state write; if there are still broadcasts in the table at that
-- point those will surface as unread to them, which matches "I'm new,
-- show me what's been announced" intent.
--
-- COALESCE handles the empty-table case (no broadcasts ever sent).
-- ON CONFLICT DO NOTHING makes re-runs idempotent.
-- ──────────────────────────────────────────
INSERT INTO system_message_read_state (user_id, last_read_message_id, updated_at)
SELECT
    u.id,
    COALESCE((SELECT MAX(id) FROM system_message), 0),
    NOW()
FROM "user" u
ON CONFLICT (user_id) DO NOTHING;

-- ──────────────────────────────────────────
-- 3. Drop the now-obsolete global status column.
--
-- Keeping it would invite drift — new code reads the cursor, but a stray
-- admin tool or old script could still flip `status` and silently lie
-- about per-user state. Removing the column makes the new contract
-- impossible to misuse.
-- ──────────────────────────────────────────
ALTER TABLE system_message DROP COLUMN IF EXISTS status;

COMMIT;
