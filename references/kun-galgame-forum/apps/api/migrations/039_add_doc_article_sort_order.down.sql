DROP INDEX IF EXISTS idx_doc_article_sort_order;
ALTER TABLE doc_article DROP COLUMN IF EXISTS sort_order;
