-- 011: bump galgame_comment.content max length to 5000
--
-- The old 1007 was inherited from the original Prisma schema and was
-- too tight once Markdown authoring landed (a few paragraphs of
-- regular text easily blow past it, before counting any code blocks /
-- math / link syntax). 5000 leaves headroom for typical long-form
-- comments while still bounding the row + caching cost.
--
-- PostgreSQL ALTER COLUMN TYPE for varchar(N) → varchar(M) where M > N
-- is a fast metadata-only operation (no row rewrite), so this runs
-- instantly even on the live table.

BEGIN;

ALTER TABLE galgame_comment
    ALTER COLUMN content TYPE varchar(5000);

COMMIT;
