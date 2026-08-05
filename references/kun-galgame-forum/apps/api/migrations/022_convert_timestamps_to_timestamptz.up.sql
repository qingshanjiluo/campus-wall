-- 022: convert naive `timestamp` columns to `timestamptz`, per their real zone.
--
-- THE BUG. `kungal-api` runs in UTC (it has no TZ env; only `web` sets
-- TZ=Asia/Shanghai). GORM writes `time.Now()` (a UTC wall clock) into naive
-- `timestamp without time zone` columns. The DB session is pinned to
-- Asia/Shanghai (postgres.go), so on read those UTC wall clocks get `+08:00`
-- stapled on — every timestamp surfaces 8 hours early. (The frontend is
-- innocent: `new Date(wire)` faithfully parses whatever instant the server
-- encodes.) Verified against the live clock: topic.updated, bumped on every
-- view, tracks UTC-now to the second, not Beijing-now.
--
-- THE FIX. `timestamptz` stores an absolute instant, so the api's process zone
-- stops mattering — the wire always carries the right moment and each viewer
-- renders it locally. This also fixes a latent risk on the distroless api
-- (no embedded tzdata, so a bare TZ env wouldn't even take effect).
--
-- WHY PER-COLUMN, NOT A UNIFORM CONVERSION. The stored values are NOT all in one
-- zone — audited write-path by write-path (code + live data):
--   • 133 non-chat columns: GORM time.Now() → UTC wall clock. Reinterpret AS UTC.
--   • galgame_resource.update_time: no GORM field maps it, so the column DEFAULT
--     (CURRENT_TIMESTAMP, evaluated in the Asia/Shanghai session) writes a
--     BEIJING wall clock. Reinterpret AS Asia/Shanghai (phase 2).
--   • the chat subsystem (chat_* tables): chat_repo.go INSERTs with Postgres
--     NOW() (Beijing) but UPDATEs with Go `?`=time.Now() (UTC) — the SAME column
--     holds both zones per-row (e.g. chat_room had rows with updated<created).
--     No single conversion can be correct, so chat is LEFT NAIVE here. Fix the
--     chat code to a single clock, then migrate it separately.
--   • _migrations.applied_at: migrator bookkeeping (Beijing default) — left alone.
--   • columns already timestamptz (friend_link.*, galgame_activity.created,
--     *.edited, *_read_state.updated_at): skipped by the data_type filter.
--
-- SAFETY. Pre-checked: no views / matviews / generated columns depend on these.
-- Per-column precision is preserved. Phase 3 is a guard: if any column we
-- converted as UTC was actually Beijing (a DB-default we failed to spot), its
-- instant is now +8h and reads as the future — the migration RAISEs and the
-- whole BEGIN/COMMIT rolls back rather than silently corrupting data.
--
-- AFTER DEPLOY: restart kungal-api. The type change invalidates pgx's per-conn
-- cached statement plans ("cached plan must not change result type"); a restart
-- rebuilds them cleanly.
BEGIN;

-- ── Phase 1: UTC-written columns (everything except chat, the Beijing default,
--    and the migrator table). Reinterpret naive wall clock AS UTC. ──
DO $$
DECLARE r record;
BEGIN
  FOR r IN
    SELECT c.table_name, c.column_name, c.datetime_precision
    FROM information_schema.columns c
    JOIN information_schema.tables t
      ON t.table_schema = c.table_schema AND t.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND t.table_type   = 'BASE TABLE'
      AND c.data_type    = 'timestamp without time zone'
      AND c.table_name  <> '_migrations'
      AND c.table_name NOT LIKE 'chat\_%'
      AND NOT (c.table_name = 'galgame_resource' AND c.column_name = 'update_time')
    ORDER BY c.table_name, c.column_name
  LOOP
    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE timestamptz(%s) USING %I AT TIME ZONE %L',
      r.table_name, r.column_name, r.datetime_precision, r.column_name, 'UTC');
  END LOOP;
END $$;

-- ── Phase 2: the lone Beijing-written column (DB DEFAULT in Asia/Shanghai).
--    Guarded so a manual re-run can't double-convert an already-timestamptz col. ──
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'galgame_resource'
      AND column_name = 'update_time' AND data_type = 'timestamp without time zone'
  ) THEN
    ALTER TABLE galgame_resource
      ALTER COLUMN update_time TYPE timestamptz(3) USING update_time AT TIME ZONE 'Asia/Shanghai';
  END IF;
END $$;

-- ── Phase 3: abort if any converted column is implausibly future-dated, which
--    would mean it was Beijing-stored and got the wrong (UTC) reinterpretation.
--    `deadline` (poll end times) is legitimately in the future and is exempt. ──
DO $$
DECLARE r record; v timestamptz; bad text := '';
BEGIN
  FOR r IN
    SELECT c.table_name, c.column_name
    FROM information_schema.columns c
    JOIN information_schema.tables t
      ON t.table_schema = c.table_schema AND t.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND t.table_type   = 'BASE TABLE'
      AND c.data_type    = 'timestamp with time zone'
      AND c.column_name <> 'deadline'
  LOOP
    EXECUTE format('SELECT max(%I) FROM public.%I', r.column_name, r.table_name) INTO v;
    IF v IS NOT NULL AND v > now() + interval '1 hour' THEN
      bad := bad || format('%s.%s=%s ', r.table_name, r.column_name, v);
    END IF;
  END LOOP;
  IF bad <> '' THEN
    RAISE EXCEPTION 'timestamptz conversion aborted — future-dated (Beijing-not-UTC?) columns: %', bad;
  END IF;
END $$;

COMMIT;
