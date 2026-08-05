-- Per-user notification preferences: an opt-out set of muted category keys.
-- Stored as a flat jsonb array (default '[]') on kungal_user_state so the
-- state row already loaded by GetUserStatus carries the muted set for free.
-- Keys are message.type values (upvoted/liked/…/quiz-answered), stream pseudo
-- keys (system/chat), and namespaced wiki keys (wiki:approved/…). Muting only
-- suppresses the red dot / unread badges — the message rows are still kept and
-- remain visible in the notification center. Idempotent.
ALTER TABLE kungal_user_state
  ADD COLUMN IF NOT EXISTS muted_notification_types jsonb NOT NULL DEFAULT '[]'::jsonb;
