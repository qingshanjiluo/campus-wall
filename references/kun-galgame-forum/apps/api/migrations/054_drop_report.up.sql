-- Remove the legacy anonymous "report" table. It was a write-only dead-drop
-- (no reporter id, no target reference, no reviewer surface); content reporting
-- is being re-done against the infra Trust & Safety platform. The old report
-- data is intentionally discarded. CASCADE also drops its owned sequence.
DROP TABLE IF EXISTS report CASCADE;
