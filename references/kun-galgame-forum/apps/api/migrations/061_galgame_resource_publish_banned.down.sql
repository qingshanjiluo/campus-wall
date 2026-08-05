-- Reverse of 061: drop the galgame resource-publish ban flag.
ALTER TABLE galgame DROP COLUMN IF EXISTS resource_publish_banned;
