-- 024: fix chat_room.updated for never-messaged rooms (8h-off after 023).
--
-- chat_room.updated is the one genuinely zone-mixed column: written by Postgres
-- NOW() (Beijing) on insert AND Go time.Now() (UTC) on every message. 023
-- converted it AS UTC — correct for the active majority (899 rooms), but for
-- rooms that were NEVER messaged, `updated` only ever held the NOW() insert
-- value (the same Beijing wall clock as `created`). AS-UTC conversion therefore
-- landed it EXACTLY 8h after the (correctly AS-Asia/Shanghai-converted) created.
--
-- A never-touched room's updated should equal its created, so restore that.
-- Targets only the proven-wrong rows (last_message_time IS NULL AND the exact
-- +8h signature — 239 rows on prod, 0 false positives). Idempotent: after the
-- fix updated = created, so the condition no longer matches on re-run.
BEGIN;

UPDATE chat_room
SET updated = created
WHERE last_message_time IS NULL
  AND updated = created + interval '8 hours';

COMMIT;
