package userclient

import "context"

// CollectIDs walks `rows`, applies `idOf` to extract a user_id from each,
// and returns a deduplicated slice (skipping zero / negative ids). Used at
// the top of every mapper that needs to enrich rows with identity.
func CollectIDs[T any](rows []T, idOf func(T) int) []int {
	if len(rows) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(rows))
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		id := idOf(r)
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// DerefID reads a NULLABLE identity column (e.g. the frozen wiki-era galgame
// creator snapshot, migration 066) as a plain user id, collapsing "unknown" to
// 0. That 0 is exactly what the rest of this package treats as "no user":
// CollectIDs skips it, so Hydrate is never asked about it, so the lookup misses
// and yields the zero User. Rendering an unknown author therefore produces an
// empty chip rather than the 已注销用户 placeholder Hydrate mints for every id
// it IS asked about.
func DerefID(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// Hydrate looks up `ids` and returns a *total* map (every id in `ids` is
// present, missing/unknown ids get a Placeholder). Mapper code can then do
// `m[r.UserID]` without nil-checks — either it's a real user or it's a
// placeholder rendered as "已注销用户". Errors from OAuth are folded into
// placeholders so a transient OAuth outage degrades gracefully (nobody
// loses identity rendering, names just temporarily show as placeholders).
//
// Banned users (status != 0) are returned as-is. Per the agreed policy
// kungal hides banned users at the mapper layer; mappers must still call
// IsRenderable(u) themselves where filtering applies.
func (c *Client) Hydrate(ctx context.Context, ids []int) map[int]User {
	out := make(map[int]User, len(ids))
	if len(ids) == 0 {
		return out
	}
	users, _ := c.Users(ctx, ids)
	for _, id := range ids {
		if u, ok := users[id]; ok {
			out[id] = u
		} else {
			out[id] = Placeholder(id)
		}
	}
	return out
}

// IsRenderable reports whether a user's content should be shown in lists.
// Returns false for status != 0 (banned/deleted) per the agreed policy.
// Use at mapper layer to filter out rows authored by banned users.
func IsRenderable(u User) bool {
	return u.Status == 0
}
