-- Recreate the column (nullable, empty) — historical target values are NOT
-- restored, matching the reply-target retirement (028) down migration.
ALTER TABLE public.galgame_comment ADD COLUMN IF NOT EXISTS target_user_id integer;
