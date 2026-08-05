-- 019 (down): intentional no-op.
--
-- The dropped FKs referenced the obsolete public."user" table and rejected every
-- post-OAuth-cutover user. Re-creating them would re-introduce that bug and would
-- fail anyway once a post-cutover user has inserted an interaction row (its
-- user_id isn't in `user`). Identity integrity now lives in OAuth -- nothing to
-- restore.

DO $$ BEGIN
  RAISE NOTICE '019 down is a no-op: obsolete user FKs are intentionally not restored';
END $$;
