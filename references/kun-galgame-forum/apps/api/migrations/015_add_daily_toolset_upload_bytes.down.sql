BEGIN;

ALTER TABLE kungal_user_state
    DROP COLUMN IF EXISTS daily_toolset_upload_bytes;

COMMIT;
