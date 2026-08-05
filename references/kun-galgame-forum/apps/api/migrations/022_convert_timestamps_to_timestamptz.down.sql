-- 022 down: revert the converted columns back to naive `timestamp` (restoring
-- the exact wall clock each one held before 022), and leave everything else
-- untouched.
--
-- Inverse of the up conversion, zone-for-zone:
--   • galgame_resource.update_time  → naive via AT TIME ZONE 'Asia/Shanghai'
--   • all other converted columns   → naive via AT TIME ZONE 'UTC'
--
-- EXCLUDES the columns that were already timestamptz BEFORE 022 (created that
-- way by migrations 008/010/012/014/017/021) — downgrading them would corrupt
-- them. Chat tables were never converted (still naive), so the data_type filter
-- skips them automatically.
BEGIN;

-- Revert the Beijing column first so the UTC loop below doesn't also catch it.
-- Guarded so a manual re-run can't double-convert an already-naive column.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'galgame_resource'
      AND column_name = 'update_time' AND data_type = 'timestamp with time zone'
  ) THEN
    ALTER TABLE galgame_resource
      ALTER COLUMN update_time TYPE timestamp(3) without time zone
      USING update_time AT TIME ZONE 'Asia/Shanghai';
  END IF;
END $$;

DO $$
DECLARE
  r record;
  already_tz text[] := ARRAY[
    'friend_link.created',
    'friend_link.updated',
    'galgame_activity.created',
    'galgame_comment.edited',
    'system_message_read_state.updated_at',
    'topic_comment.edited',
    'wiki_message_read_state.updated_at'
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
      AND (c.table_name || '.' || c.column_name) <> ALL (already_tz)
    ORDER BY c.table_name, c.column_name
  LOOP
    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE timestamp(%s) without time zone USING %I AT TIME ZONE %L',
      r.table_name, r.column_name, r.datetime_precision, r.column_name, 'UTC');
  END LOOP;
END $$;

COMMIT;
