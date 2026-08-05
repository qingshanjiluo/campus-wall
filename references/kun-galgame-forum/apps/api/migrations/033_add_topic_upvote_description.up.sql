-- The optional one-liner a user may write when pushing (推) a topic — "why I
-- pushed it" — shown on the 推话题 activity card. <=30 chars; '' when omitted
-- (the card then shows a random default blurb). Idempotent.
ALTER TABLE topic_upvote
    ADD COLUMN IF NOT EXISTS description VARCHAR(30) NOT NULL DEFAULT '';
