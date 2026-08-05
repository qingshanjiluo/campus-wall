-- 027 down: narrow message.content back to varchar(233). Any row longer than 233
-- chars (a tokenized snapshot written after 027) is truncated with left(content,
-- 233) to fit the narrower type — lossy by the nature of the revert.
BEGIN;

ALTER TABLE message
  ALTER COLUMN content TYPE varchar(233) USING left(content, 233);

COMMIT;
