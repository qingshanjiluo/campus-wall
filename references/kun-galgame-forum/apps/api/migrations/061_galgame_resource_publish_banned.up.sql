-- galgame.resource_publish_banned: moderator kill-switch that forbids publishing
-- (and editing) download resources under a galgame. Set when a title becomes
-- unavailable for a copyright-holder notice or other third-party reason. The
-- resource BFF (resource_service.go CreateResource / UpdateResource) rejects with
-- 403 while true; DELETE stays allowed so existing resources can still be cleaned
-- up. Toggled by moderators via PUT /admin/galgame/:gid/resource-publish-ban.
-- Additive + idempotent, so it auto-runs on deploy (NOT in the migrate --exclude list).
ALTER TABLE galgame ADD COLUMN IF NOT EXISTS resource_publish_banned boolean NOT NULL DEFAULT false;
