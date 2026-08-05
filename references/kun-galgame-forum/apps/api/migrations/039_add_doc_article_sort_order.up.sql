-- Manual display order for doc articles (admin drag-and-drop reorder).
-- Lower sort_order = shown earlier. The public doc list + the admin manager
-- both order by this column.

ALTER TABLE doc_article
  ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_doc_article_sort_order
  ON doc_article (sort_order, id);

-- One-time seed by publish time (newest first → smallest sort_order).
-- Guarded so it only runs while EVERY row is still at the default 0 (i.e. the
-- order has never been touched): once an admin has dragged anything, a re-run
-- is a no-op and will not clobber their manual order. (The migration runner
-- already applies each file once; this guard just makes a manual re-run safe.)
UPDATE doc_article AS a
SET sort_order = s.pos
FROM (
  SELECT id, (ROW_NUMBER() OVER (ORDER BY published_time DESC, id DESC) - 1) AS pos
  FROM doc_article
) AS s
WHERE a.id = s.id
  AND NOT EXISTS (SELECT 1 FROM doc_article WHERE sort_order <> 0);
