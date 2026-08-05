-- 031: Telegram-style reactions for topics + replies.
--
-- Unifies 点赞(like) + 点踩(dislike) into one reaction table per target
-- (topic / reply). `reaction` is a key: 'like' / 'dislike' (effectful — like still
-- grants the content owner +1 moemoepoint) or an emoji key ('fire', 'heart', …).
-- A user may hold several different reactions on one target (Discord-style);
-- like/dislike mutual exclusion is enforced in the service, not the schema.
--
-- like/dislike COUNTS stay denormalized on topic / topic_reply (the activity feed
-- + profile stats read them); emoji reaction counts are aggregated on read. The
-- legacy topic_like / topic_dislike / topic_reply_like / topic_reply_dislike
-- tables are BACKFILLED here and kept for now — a later migration drops them once
-- the code has switched over (deploy-then-drop, the reverse of the usual order).
-- Idempotent.
BEGIN;

CREATE TABLE IF NOT EXISTS public.topic_reaction (
  id       SERIAL PRIMARY KEY,
  topic_id INTEGER NOT NULL,
  user_id  INTEGER NOT NULL,
  reaction VARCHAR(32) NOT NULL,
  created  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_topic_reaction UNIQUE (topic_id, user_id, reaction)
);
CREATE INDEX IF NOT EXISTS idx_topic_reaction_topic ON public.topic_reaction (topic_id);
CREATE INDEX IF NOT EXISTS idx_topic_reaction_user_reaction ON public.topic_reaction (user_id, reaction);

CREATE TABLE IF NOT EXISTS public.topic_reply_reaction (
  id             SERIAL PRIMARY KEY,
  topic_reply_id INTEGER NOT NULL,
  user_id        INTEGER NOT NULL,
  reaction       VARCHAR(32) NOT NULL,
  created        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_topic_reply_reaction UNIQUE (topic_reply_id, user_id, reaction)
);
CREATE INDEX IF NOT EXISTS idx_topic_reply_reaction_reply ON public.topic_reply_reaction (topic_reply_id);
CREATE INDEX IF NOT EXISTS idx_topic_reply_reaction_user_reaction ON public.topic_reply_reaction (user_id, reaction);

-- Backfill legacy like/dislike into the unified tables (idempotent).
INSERT INTO public.topic_reaction (topic_id, user_id, reaction, created)
  SELECT topic_id, user_id, 'like', created FROM public.topic_like
  ON CONFLICT (topic_id, user_id, reaction) DO NOTHING;
INSERT INTO public.topic_reaction (topic_id, user_id, reaction, created)
  SELECT topic_id, user_id, 'dislike', created FROM public.topic_dislike
  ON CONFLICT (topic_id, user_id, reaction) DO NOTHING;

INSERT INTO public.topic_reply_reaction (topic_reply_id, user_id, reaction, created)
  SELECT topic_reply_id, user_id, 'like', created FROM public.topic_reply_like
  ON CONFLICT (topic_reply_id, user_id, reaction) DO NOTHING;
INSERT INTO public.topic_reply_reaction (topic_reply_id, user_id, reaction, created)
  SELECT topic_reply_id, user_id, 'dislike', created FROM public.topic_reply_dislike
  ON CONFLICT (topic_reply_id, user_id, reaction) DO NOTHING;

COMMIT;
