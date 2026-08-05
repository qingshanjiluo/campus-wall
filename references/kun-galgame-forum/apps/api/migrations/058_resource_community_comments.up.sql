-- Resource comment migration onto the community primitive (charter step 07).
-- Additive and safe to deploy BEFORE the cutover: with the feature flag off
-- (KUN_RESOURCE_COMMENTS_COMMUNITY unset) the table sits idle.

-- resource_comment_community_map: old (source, id) -> its migrated community
-- (thread, post), for audit + the 06a old-world retirement reconciliation
-- (charter ruling 24 — the three resource areas carry no old likes and no deep
-- links, so the map is not read by the BFF). `source` multiplexes the three
-- areas: 'rating' / 'website' / 'toolset'. ALSO created by the infra import tool
-- (cmd/import-kungal-resource-comments, step 06) with the IDENTICAL
-- CREATE TABLE IF NOT EXISTS DDL, so adopting it here is idempotent whichever
-- side runs first. NOT back-filled here: the map is populated by the import tool
-- at cutover.
CREATE TABLE IF NOT EXISTS resource_comment_community_map (
  source    varchar(16) NOT NULL,
  old_id    int         NOT NULL,
  thread_id bigint      NOT NULL,
  post_id   bigint      NOT NULL,
  PRIMARY KEY (source, old_id)
);
