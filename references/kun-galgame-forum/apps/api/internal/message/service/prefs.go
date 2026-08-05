package service

import "strings"

// This file owns the notification-preference key vocabulary: which category
// keys a user may mute, and how a stored muted set is split into the pieces the
// server enforces. Muting only suppresses the red dot / unread badges — rows
// are still written and stay visible in the notification center (see migration
// 053). The keys come in three flavours:
//
//   - Local types  — the exact message.type values (upvoted/liked/…). Muting
//     these excludes their unread rows from CountUnreadMessages / the nav badge.
//   - Stream pseudo key — "chat" (private messages), enforced by zeroing its
//     unread count. (Official system/admin broadcasts are intentionally NOT
//     mutable — there's no key for them.)
//   - Wiki keys    — "wiki:*" namespaced (avoids colliding with the local
//     "declined"). Wiki never feeds has_new_message, so it's filtered on the
//     frontend; the server only validates + stores these.
const (
	KeyChat    = "chat"
	wikiPrefix = "wiki:"
)

// LocalNotificationTypes are the message.type values a user may mute. Note it
// includes "quiz-answered", which is emitted as a hardcoded string and is NOT
// part of the NotifyKind enum in notifier.go. "admin" is intentionally omitted:
// it's an internal label, not a user-facing category.
var LocalNotificationTypes = []string{
	string(NotifyUpvoted), string(NotifyLiked), string(NotifyFavorite),
	string(NotifyReplied), string(NotifyCommented), string(NotifyMentioned),
	string(NotifySolution), string(NotifyPinReply), string(NotifyExpired),
	string(NotifyRequested), string(NotifyMerged), string(NotifyDeclined),
	"quiz-answered",
}

// WikiNotificationKeys are the namespaced wiki-review categories.
var WikiNotificationKeys = []string{
	"wiki:approved", "wiki:declined", "wiki:banned", "wiki:unbanned",
}

// allNotificationKeys is the PUT whitelist: unknown keys are rejected so junk
// never reaches the muted set.
var allNotificationKeys = func() map[string]bool {
	m := make(map[string]bool)
	for _, k := range LocalNotificationTypes {
		m[k] = true
	}
	m[KeyChat] = true
	for _, k := range WikiNotificationKeys {
		m[k] = true
	}
	return m
}()

// SanitizeMutedKeys drops unknown/duplicate keys, returning a clean muted set
// safe to persist. A nil result is normalised to an empty slice by the caller.
func SanitizeMutedKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if allNotificationKeys[k] && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// SplitMuted classifies a (already-validated) muted set into the local
// message.type subset and the chat-stream flag the server enforces. Wiki keys
// are handled on the frontend and ignored here.
func SplitMuted(muted []string) (local []string, chatMuted bool) {
	for _, k := range muted {
		switch {
		case k == KeyChat:
			chatMuted = true
		case strings.HasPrefix(k, wikiPrefix):
			// wiki:* — enforced on the frontend.
		default:
			local = append(local, k)
		}
	}
	return local, chatMuted
}
