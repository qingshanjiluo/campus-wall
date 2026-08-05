-- Indexes for the home 话题/资源求助 tabs, which are a TOPIC LIST sorted by the
-- topic's last-activity time (status_update_time) — not the event-time feed.
--   * idx_topic_status_update_time: the feed sort + keyset (status_update_time, id).
--   * idx_topic_reply/comment_topic_created: the "latest reply/comment" card
--     enrichment (DISTINCT ON (topic_id) ORDER BY created DESC, scoped to a page's
--     topic ids).
CREATE INDEX IF NOT EXISTS idx_topic_status_update_time
  ON topic (status_update_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_topic_reply_topic_created
  ON topic_reply (topic_id, created DESC);

CREATE INDEX IF NOT EXISTS idx_topic_comment_topic_created
  ON topic_comment (topic_id, created DESC);
