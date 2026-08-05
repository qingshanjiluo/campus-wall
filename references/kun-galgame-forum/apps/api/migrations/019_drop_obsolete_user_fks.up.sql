-- 019: Drop the FK constraints that still reference the obsolete public."user".
--
-- Background: after the OAuth migration, identity lives in OAuth and the local
-- user mirror moved to kungal_user_state (keyed user_id = OAuth user.id, created
-- via StateRepository.Ensure() at the OAuth callback). Nothing writes the legacy
-- `user` table anymore. But 55 tables still FK their user columns (user_id /
-- sender_id / receiver_id / follower_id / followed_id / target_user_id /
-- author_id / admin_user_id) at public."user"(id). So any user who registered
-- AFTER the OAuth cutover -- present in OAuth + kungal_user_state, absent from
-- `user` -- hit those FKs on their first favorite/like/comment/message/follow:
-- 23503 foreign_key_violation, which aborts the tx and cascades into 25P02 on
-- every following statement (see interaction_repo.go ToggleFavorite).
--
-- These FKs enforce nothing real: the referenced table is a stale mirror, not
-- the OAuth source of truth, and user_id is already validated by the JWT. Their
-- ON DELETE/UPDATE CASCADE is inert (the app no longer mutates `user`; user
-- deletion / content purge is app-side). Dropping them needs NO data migration:
-- the FK was BLOCKING the bad inserts, so those tx's already rolled back -- there
-- are no orphan rows to clean.
--
-- Done dynamically so it covers every inbound FK regardless of constraint name.

BEGIN;

DO $$
DECLARE
  r RECORD;
  n INT := 0;
BEGIN
  FOR r IN
    SELECT conrelid::regclass AS tbl, conname
    FROM pg_constraint
    WHERE contype = 'f'
      AND confrelid = to_regclass('public."user"')
  LOOP
    EXECUTE format('ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I', r.tbl, r.conname);
    n := n + 1;
  END LOOP;
  RAISE NOTICE '019: dropped % obsolete user-referencing FK constraint(s)', n;
END $$;

COMMIT;
