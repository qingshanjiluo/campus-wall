-- 017 down: drop the friend_link table.
-- Friend links revert to the static apps/web/app/config/friend.json fallback
-- only if the frontend is also rolled back; otherwise the list renders empty.
BEGIN;

DROP TABLE IF EXISTS friend_link;

COMMIT;
