-- 025 down: narrow galgame_resource.note back to varchar(1007). Any note longer
-- than 1007 chars (rich-markdown notes written after 025) is truncated with
-- left(note, 1007) to fit the narrower type — lossy by the nature of the revert.
BEGIN;

ALTER TABLE galgame_resource
  ALTER COLUMN note TYPE varchar(1007) USING left(note, 1007);

COMMIT;
