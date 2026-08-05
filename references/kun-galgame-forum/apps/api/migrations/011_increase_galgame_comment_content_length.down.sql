-- 011 down: shrink content back to varchar(1007).
--
-- Will FAIL if any row currently exceeds 1007 chars — that's
-- intentional. Manually triage the offending rows before rolling
-- back this migration.

BEGIN;

ALTER TABLE galgame_comment
    ALTER COLUMN content TYPE varchar(1007);

COMMIT;
