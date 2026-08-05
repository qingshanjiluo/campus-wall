-- 030 down: recreate the unmoe table (EMPTY) with its original structure, so an
-- older code revision that still expects it can run. The dropped rows are NOT
-- restored — this only reverts the schema change.
BEGIN;

CREATE TABLE IF NOT EXISTS public.unmoe (
    id integer NOT NULL,
    name text NOT NULL,
    result text DEFAULT ''::text NOT NULL,
    desc_en_us text DEFAULT ''::text NOT NULL,
    desc_ja_jp text DEFAULT ''::text NOT NULL,
    desc_zh_cn text DEFAULT ''::text NOT NULL,
    desc_zh_tw text DEFAULT ''::text NOT NULL,
    user_id integer NOT NULL,
    created timestamp(3) without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated timestamp(3) without time zone NOT NULL
);

CREATE SEQUENCE IF NOT EXISTS public.unmoe_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
ALTER SEQUENCE public.unmoe_id_seq OWNED BY public.unmoe.id;
ALTER TABLE ONLY public.unmoe
    ALTER COLUMN id SET DEFAULT nextval('public.unmoe_id_seq'::regclass);

ALTER TABLE ONLY public.unmoe
    ADD CONSTRAINT unmoe_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.unmoe
    ADD CONSTRAINT unmoe_user_id_fkey FOREIGN KEY (user_id)
    REFERENCES public."user"(id) ON UPDATE CASCADE ON DELETE CASCADE;

COMMIT;
