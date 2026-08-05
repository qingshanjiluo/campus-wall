package catalogclient

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// EditRevisionFeedItem is one row of the editing engine's global revision
// cursor feed (GET /api/v1/catalog/edit-revisions/feed). The engine is the sole
// author of galgame field edits since E3a, so this feed — not the wiki's
// /galgame/revisions/recent — is the authoritative record of "an entity's
// fields changed".
//
// EntityID is the entity's own id in its family's key space: for
// entity_type=galgame.game that is the kungal gid, and Seq is the per-entity
// revision NUMBER the diff endpoint takes as :rev.
//
// The snapshot is deliberately not on the wire (a catch-up page would be
// megabytes); a consumer that wants field values reads the revision face for
// the rows it cares about.
type EditRevisionFeedItem struct {
	ID            int64     `json:"id"`
	EntityFamily  string    `json:"entity_family"`
	EntityType    string    `json:"entity_type"`
	EntityID      int64     `json:"entity_id"`
	Seq           int       `json:"seq"`
	Action        int16     `json:"action"`
	ChangedFields []string  `json:"changed_fields"`
	ActorUID      int64     `json:"actor_uid"`
	AmenderUID    *int64    `json:"amender_uid"`
	ProposalID    *int64    `json:"proposal_id"`
	Site          string    `json:"site"`
	CreatedAt     time.Time `json:"created_at"`
}

// Engine revision actions. Only a HUMAN CONTENT CHANGE belongs on an activity
// timeline: created is the entity's birth (the registry minting a row), which
// downstream products already learn about through their own creation events.
const (
	EditActionCreated  int16 = 0
	EditActionMerged   int16 = 1
	EditActionDirect   int16 = 2
	EditActionReverted int16 = 3
)

// EditRevisionFeedPage is one page of the cursor feed. NextSince echoes the
// request's cursor on an empty page, so a consumer that stores it
// unconditionally never rewinds.
type EditRevisionFeedPage struct {
	Items     []EditRevisionFeedItem `json:"items"`
	NextSince int64                  `json:"next_since"`
}

// EditRevisionsSince reads one page of the revision feed after the exclusive
// cursor `since`, optionally narrowed to one entity type. The feed is ascending
// by id and has no has_more flag: a page shorter than the limit is the tail.
func (c *Client) EditRevisionsSince(
	ctx context.Context,
	since int64,
	limit int,
	entityType string,
) (*EditRevisionFeedPage, error) {
	q := url.Values{
		"since": {strconv.FormatInt(since, 10)},
		"limit": {strconv.Itoa(limit)},
	}
	if entityType != "" {
		q.Set("entity_type", entityType)
	}
	data, err := c.getData(ctx, "/api/v1/catalog/edit-revisions/feed", q)
	if err != nil {
		return nil, err
	}
	var page EditRevisionFeedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, err
	}
	return &page, nil
}
