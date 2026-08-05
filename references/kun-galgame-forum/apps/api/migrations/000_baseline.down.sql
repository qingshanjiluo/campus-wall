-- 000_baseline.down.sql
--
-- Intentionally a no-op. Rolling back the baseline means dropping every
-- table the application owns, which is never the right answer in any
-- realistic scenario — wipe the database manually if that is genuinely
-- what you want, e.g. via:
--
--     DROP SCHEMA public CASCADE; CREATE SCHEMA public;
--
-- and then re-run `pnpm migrate` from a fresh state.

SELECT 1;
