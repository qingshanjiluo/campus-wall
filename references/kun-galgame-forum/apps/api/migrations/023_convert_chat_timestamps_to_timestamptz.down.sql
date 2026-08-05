-- 023 down: revert the chat columns back to naive `timestamp`, restoring each
-- one's original wall clock (inverse of the up conversion, zone-for-zone).
-- Chat is the only place with timestamptz columns named chat_*, so the
-- data_type + name filter targets exactly what 023 converted.
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
      AND c.data_type    = 'timestamp with time zone'
      AND c.table_name LIKE 'chat\_%'
    ORDER BY c.table_name, c.column_name
  LOOP
    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE timestamp(%s) without time zone USING %I AT TIME ZONE %L',
      r.table_name, r.column_name, r.datetime_precision, r.column_name,
      CASE WHEN (r.table_name || '.' || r.column_name) = ANY (beijing)
           THEN 'Asia/Shanghai' ELSE 'UTC' END);
  END LOOP;
END $$;

COMMIT;
