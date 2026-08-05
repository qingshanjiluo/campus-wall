-- 028 down: recreate topic_reply_target (EMPTY) with its original structure, so
-- an older code revision that still expects the table can run. The dropped rows
-- are NOT restored — this only reverts the schema change.
BEGIN;

CREATE TABLE IF NOT EXISTS public.topic_reply_target (
    id integer NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    reply_id integer NOT NULL,
    target_reply_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

CREATE SEQUENCE IF NOT EXISTS public.topic_reply_target_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.topic_reply_target_id_seq OWNED BY public.topic_reply_target.id;
ALTER TABLE ONLY public.topic_reply_target
    ALTER COLUMN id SET DEFAULT nextval('public.topic_reply_target_id_seq'::regclass);

ALTER TABLE ONLY public.topic_reply_target
    ADD CONSTRAINT topic_reply_target_pkey PRIMARY KEY (id);
CREATE UNIQUE INDEX IF NOT EXISTS topic_reply_target_reply_id_target_reply_id_key
    ON public.topic_reply_target USING btree (reply_id, target_reply_id);
ALTER TABLE ONLY public.topic_reply_target
    ADD CONSTRAINT topic_reply_target_reply_id_fkey FOREIGN KEY (reply_id)
    REFERENCES public.topic_reply(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE ONLY public.topic_reply_target
    ADD CONSTRAINT topic_reply_target_target_reply_id_fkey FOREIGN KEY (target_reply_id)
    REFERENCES public.topic_reply(id) ON UPDATE CASCADE ON DELETE CASCADE;

COMMIT;
