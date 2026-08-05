-- 027: widen message.content from varchar(233) to text.
--
-- Notification rows snapshot a copy of the topic/reply content that triggered
-- them, including embedded images. The old image-bed URLs are PATH-based and
-- short (~58 chars, e.g. image.kungal.com/topic/user_2/鲲-<ts>.webp), but the
-- domain-independent token they migrate to — /image/<64-hex-hash> — is 71 chars.
-- On a notification already near the 233 cap, rewriting a short URL to a longer
-- token overflowed varchar(233) (SQLSTATE 22001), so backfill-content-images
-- could not migrate ~half the message rows.
--
-- text aligns message.content with every other markdown content column
-- (topic/reply/chat/comment/target are all text). New notifications are still
-- bounded by the notifier's 233-char truncation (notifier.go notifyn) — this
-- only relaxes the column so existing tokenized snapshots fit. varchar(233) ->
-- text only relaxes the length check; Postgres does NOT rewrite the table, so
-- this is fast and safe on prod.
BEGIN;

ALTER TABLE message
  ALTER COLUMN content TYPE text;

COMMIT;
