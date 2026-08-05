-- 024 down: intentional no-op. The pre-024 value was the 8h-wrong timestamp;
-- restoring it would re-introduce the bug and the original is not kept. The
-- migrator still records/removes the version row around this.
SELECT 1;
