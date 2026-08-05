package service

import (
	"context"
	"sync"
	"time"
)

// communityStatsTTL is how long a per-user community visible_posts count is
// cached. The floating hover card fires on every hover, so a short in-process
// memo keeps the S2S face from being hammered while staying fresh enough for a
// contribution counter (a few minutes of staleness is harmless).
const communityStatsTTL = 7 * time.Minute

// visiblePostsCache is a tiny mutex-guarded TTL cache of per-user community
// visible_posts. It lives on UserService (a singleton), NOT in communityclient
// (which stays stateless). On an S2S error the last good value is served if
// present (never expired away), so a transient outage degrades to a slightly
// stale count rather than a zero flicker.
type visiblePostsCache struct {
	mu      sync.Mutex
	entries map[int]visiblePostsEntry
}

type visiblePostsEntry struct {
	count   int64
	expires time.Time
}

func newVisiblePostsCache() *visiblePostsCache {
	return &visiblePostsCache{entries: make(map[int]visiblePostsEntry)}
}

// get returns a fresh cached value (ok=true) or the stale/absent state.
func (c *visiblePostsCache) get(userID int) (count int64, fresh bool, cached bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[userID]
	if !ok {
		return 0, false, false
	}
	return e.count, time.Now().Before(e.expires), true
}

func (c *visiblePostsCache) set(userID int, count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[userID] = visiblePostsEntry{count: count, expires: time.Now().Add(communityStatsTTL)}
}

// communityVisiblePosts returns the user's community visible_posts count — the
// "all community comment areas" contribution total — memoized with a short TTL.
// Best-effort: an S2S error (or an unconfigured client) falls back to the last
// cached value, else 0, and never fails the caller.
func (s *UserService) communityVisiblePosts(ctx context.Context, userID int) int64 {
	if count, fresh, _ := s.commentCache.get(userID); fresh {
		return count
	}
	resp, err := s.community.AuthorStats(ctx, []int64{int64(userID)})
	if err != nil {
		if stale, _, ok := s.commentCache.get(userID); ok {
			return stale // serve the last good value through a transient outage
		}
		return 0
	}
	var count int64
	for _, st := range resp.Stats {
		if st.AuthorID == int64(userID) {
			count = st.VisiblePosts
			break
		}
	}
	s.commentCache.set(userID, count)
	return count
}
