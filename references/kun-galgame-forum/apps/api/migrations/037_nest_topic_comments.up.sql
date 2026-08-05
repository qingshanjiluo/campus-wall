-- 037: nested topic comments.
-- A comment may reply to another comment: parent_comment_id → topic_comment.id
-- (NULL = top-level, attached to the reply directly). ON DELETE SET NULL so
-- deleting a parent re-roots its replies to top-level instead of cascade-dropping
-- them — and never desyncs the reply/topic comment_count (the app deletes one row
-- at a time, counting it). The full reply tree is stored even though the UI
-- renders a single level — future-proof: deeper rendering later needs no
-- re-migration. Idempotent.
ALTER TABLE topic_comment
  ADD COLUMN IF NOT EXISTS parent_comment_id INTEGER;

DO $$ BEGIN
  ALTER TABLE topic_comment
    ADD CONSTRAINT fk_topic_comment_parent
    FOREIGN KEY (parent_comment_id) REFERENCES topic_comment (id) ON DELETE SET NULL;
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_topic_comment_parent
  ON topic_comment (parent_comment_id)
  WHERE parent_comment_id IS NOT NULL;
