-- 023: convert the chat subsystem's naive `timestamp` columns to `timestamptz`.
--
-- 022 deliberately skipped chat: chat_repo.go mixed Postgres NOW() (Beijing
-- session) INSERTs with Go time.Now() (UTC) UPDATEs into the same columns, so
-- the data is zone-mixed per column. Audited per-column (code + live data):
--
--   BEIJING (NOW() inserts / DB default) → reinterpret AS Asia/Shanghai:
--     chat_room.created
--     chat_room_participant.created, chat_room_participant.updated
--     chat_message_read_by.created, .updated, .read_time
--
--   UTC (Go time.Now()) → reinterpret AS UTC:
--     chat_message.created, .updated, .recall_time, .edit_time(empty)
--     chat_room.last_message_time
--     chat_room.updated  ← NOW() insert + time.Now() update; recent/active rows
--                           are UTC and dominate, so AS UTC. The few old
--                           never-messaged rooms (updated==created, Beijing)
--                           shift -8h — acceptable for ephemeral chat.
--     chat_message_reaction.*, chat_room_admin.* ← empty/legacy, zone moot.
--
-- Once timestamptz, the api's process zone no longer matters (absolute instants),
-- so the existing mixed-clock chat code becomes correct as-is; the companion
-- chat_repo.go cleanup (standardise on time.Now()) is then pure hygiene.
--
-- After running: restart kungal-api (type change invalidates pgx cached plans).
BEGIN;

DO $$
DECLARE
  r record;
  beijing text[] := ARRAY[
    'chat_room.created',
    'chat_room_participant.created', 'chat_room_participant.updated',
    'chat_message_read_by.created', 'chat_message_read_by.updated', 'chat_message_read_by.read_time'
  ];
BEGIN
  FOR r IN
    SELECT c.table_name, c.column_name, c.datetime_precision
    FROM information_schema.columns c
    JOIN information_schema.tables t
      ON t.table_schema = c.table_schema AND t.table_name = c.table_name
    WHERE c.table_schema = 'public'
      AND t.table_type   = 'BASE TABLE'
      AND c.data_type    = 'timestamp without time zone'
      AND c.table_name LIKE 'chat\_%'
    ORDER BY c.table_name, c.column_name
  LOOP
    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE timestamptz(%s) USING %I AT TIME ZONE %L',
      r.table_name, r.column_name, r.datetime_precision, r.column_name,
      CASE WHEN (r.table_name || '.' || r.column_name) = ANY (beijing)
           THEN 'Asia/Shanghai' ELSE 'UTC' END);
  END LOOP;
END $$;

-- Safety: a misclassified column (UTC-treated but actually Beijing) is now +8h
-- and reads as the future. Abort + roll back rather than corrupt.
DO $$
DECLARE r record; v timestamptz; bad text := '';
BEGIN
  FOR r IN
    SELECT c.table_name, c.column_name
    FROM information_schema.columns c
    WHERE c.table_schema = 'public'
      AND c.data_type    = 'timestamp with time zone'
      AND c.table_name LIKE 'chat\_%'
  LOOP
    EXECUTE format('SELECT max(%I) FROM public.%I', r.column_name, r.table_name) INTO v;
    IF v IS NOT NULL AND v > now() + interval '1 hour' THEN
      bad := bad || format('%s.%s=%s ', r.table_name, r.column_name, v);
    END IF;
  END LOOP;
  IF bad <> '' THEN
    RAISE EXCEPTION 'chat timestamptz conversion aborted — future-dated columns: %', bad;
  END IF;
END $$;

COMMIT;
