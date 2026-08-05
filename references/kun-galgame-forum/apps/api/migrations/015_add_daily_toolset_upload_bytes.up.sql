-- 015: per-user daily toolset upload BYTE budget
--
-- The backend tracked only daily_toolset_upload_count (a count that was
-- incremented but never enforced); the real 100MB/day limit lived ONLY in the
-- frontend (apps/web/app/config/upload.ts USER_DAILY_UPLOAD_LIMIT), so a direct
-- API caller could bypass it entirely. This column accumulates today's uploaded
-- bytes so the backend can enforce the budget at upload init. Reset to 0 every
-- day by the daily-reset cron, alongside the other daily_* counters.

BEGIN;

ALTER TABLE kungal_user_state
    ADD COLUMN IF NOT EXISTS daily_toolset_upload_bytes BIGINT NOT NULL DEFAULT 0;

COMMIT;
