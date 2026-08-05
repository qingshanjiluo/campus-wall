-- 020: Backfill topic.reply_count + topic.comment_count from the actual rows.
--
-- The new Go backend never maintained these denormalized counters: the OAuth/
-- backend migration backfilled them ONCE, and every reply/comment since drifted
-- them. Post-migration topics sat at 0 (their whole reply set is post-migration);
-- pre-migration topics undercount any replies added after the cutover. This
-- one-shot recompute corrects every existing row. reply_service/comment_service
-- now also call recomputeTopicCounts() inside each mutation tx, so the counts
-- stay correct going forward (CreateReply/DeleteReply, CreateComment/DeleteComment).
--
-- topic_comment carries topic_id directly (set in CreateComment alongside
-- topic_reply_id), so comment_count counts by topic_id — matching the runtime
-- helper exactly.

BEGIN;

UPDATE topic t SET
  reply_count   = (SELECT COUNT(*) FROM topic_reply  WHERE topic_id = t.id),
  comment_count = (SELECT COUNT(*) FROM topic_comment WHERE topic_id = t.id);

COMMIT;
