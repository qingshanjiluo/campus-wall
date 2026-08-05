package client

// The catalog public face (`{base}/v1/catalog/*`) — works browse / batch /
// detail / search / calendar. See catalog_wire.go for the wire shapes and the
// projections back onto kungal's own DTOs.
//
// The one structural cost of re-anchoring is the ID BRIDGE. kungal is keyed on
// the gid end to end (local rows, resources, ratings, collections, the
// /galgame/{gid} URL) and doc 106 R3 rules that it stays that way — no dual-id
// model, no data migration. So every call that starts from a kungal gid takes
// two hops:
//
//	gid ──POST /v1/catalog/lookup/batch──▶ catalog work id
//	    ──GET  /v1/catalog/works?ids=───▶ the row
//
// and every row that comes back maps home through `claimed_by.work_id`. The
// first hop is memoized (identity mappings essentially never change), so the
// steady state is one extra round-trip per cold batch, not per request.
//
// The first hop has TWO routes, and which one answers depends on who issued the
// gid. A wiki-era gid was issued upstream and is recorded as an external_ref
// anchor, so the anchor lookup finds it. A gid issued by the registry itself —
// every entry submitted after the switchover, where the claim adopts the work's
// own primary key — has no anchor to find, because an anchor records what some
// upstream handed out and there was no upstream. Those resolve through
// adoptedWorkIDs instead, which reads the id as a work id and keeps the row only
// if it points back. Both routes are inside catalogIDsForGIDs, so every caller
// gets both without knowing there are two.

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kun-galgame-api/pkg/errors"
)

// catalogIDsChunk is the hard per-call ceiling shared by the batch lookup and
// the works ids= filter. Both 400 above it (the deprecated face used to
// truncate silently), so the client chunks rather than letting a caller
// discover the limit as a failure.
const catalogIDsChunk = 100

// anchorSourceKeys are the catalog_source KEYS of source 12 — the provenance
// every kungal entry's external_ref anchor is filed under, and therefore the
// only handle the gid -> work id bridge has. Source 12 is being RENAMED from
// `galgame_wiki` to `curated`; its id does not move, only its key.
//
// This is a list rather than a constant so the rename needs no coordination.
// A one-value constant would have to flip in the same deploy as the infra
// rename, and between the two deploys every gid would resolve to nothing —
// which presents as "every galgame page 404s", not as anything a reader would
// trace back to a renamed label. Asking for both keys costs one extra lookup
// item per gid and is correct on either side of the rename, before it and
// after it, in either deploy order.
//
// The legacy key is removable once the rename has soaked; it is the only thing
// in this list that is temporary.
//
// NOT to be confused with the claim SITE (ClaimSiteKungal): the two happened to
// spell `galgame_wiki` alike, and stop doing so in different steps.
var anchorSourceKeys = []string{"curated", "galgame_wiki"}

// Lookup memo TTLs. A HIT is a registry identity — stable for the lifetime of
// both rows — so it is cached long. A MISS is not stable: the nightly
// wikirescue pass registers newly created wiki entries (doc 106 R6), so a miss
// must expire quickly enough that a new galgame becomes visible the same day
// rather than at the next deploy.
const (
	gidLookupHitTTL  = 30 * time.Minute
	gidLookupMissTTL = 2 * time.Minute
)

type gidLookupEntry struct {
	catalogID int64
	found     bool
	expire    time.Time
}

// catalogInclude is the include= vocabulary the kungal brief needs: the four
// localized names, the two cover slots, and the identity anchors (refs powers
// the DLsite 补票 link). Lanes that additionally render an intro / maker list
// ask for catalogDetailBriefInclude.
const (
	catalogBriefInclude       = "names,covers,refs"
	catalogDetailBriefInclude = "names,intros,labels,covers,refs"
)

// ─── the two content gates ───────────────────────────────────────────────
//
// The catalog exposes TWO independent content axes and kungal reads both. They
// were conflated once and the bill was doc 106 §38, so the distinction is
// spelled out here rather than left to the call sites:
//
//   - nsfw= is the AGE gate — may the caller see r18 WORKS at all. 94.5% of the
//     registry is r18, so closing it does not filter adult presentation, it
//     deletes the catalogue. Every kungal read lane therefore opens it
//     unconditionally; the per-work rating still rides home on the row, so the
//     R18 chip and the age-based UI keep working.
//   - content_limit= is the EDITORIAL gate — the claiming product's verdict on
//     whether an ENTRY's display material is sanitized. That is what kungal's
//     user setting has always meant, so that is what the setting now translates
//     into (see contentLimitOf in catalog_wire.go for the same split on the
//     response side).
//
// Filtering MUST stay on this side of the wire in both cases — the handbook
// forbids downstream post-filtering, and the face is the only place that can
// drop a row before it leaves the boundary.

// openPopulation opens the age gate on q. It is the WHOLE gate for the faces
// that have no editorial filter (taxonomy index / detail, entity search, the
// work detail aggregate): closing the age gate there would strip a maker's or a
// tag's entire membership, and on the detail lane it 404'd every r18 game.
func openPopulation(q url.Values) url.Values {
	q.Set("nsfw", "1")
	return q
}

// OpenPopulation is openPopulation for the service layer, which builds the
// taxonomy index queries itself.
func OpenPopulation(q url.Values) url.Values { return openPopulation(q) }

// applyWorksGate is openPopulation plus the editorial gate, for the three works
// faces that accept content_limit= (works LIST, works/search, calendar).
//
// The vocabulary is the face's own closed word list; kungal's permissive ""
// and "all" send nothing, which is "no editorial filter" — the same meaning
// they carried on the deprecated wiki face.
func applyWorksGate(q url.Values, contentLimit string) url.Values {
	openPopulation(q)
	switch contentLimit {
	case "sfw", "nsfw":
		q.Set("content_limit", contentLimit)
	}
	return q
}

// ApplyWorksGate is applyWorksGate in the isSFW form the service layer holds.
func ApplyWorksGate(q url.Values, isSFW bool) url.Values {
	return applyWorksGate(q, contentLimitFor(isSFW))
}

// contentLimitFor is the isSFW→content_limit convention shared by every public
// read path.
func contentLimitFor(isSFW bool) string {
	if isSFW {
		return "sfw"
	}
	return "all"
}

// ─── the id bridge ───────────────────────────────────────────────────────

// catalogIDsForGIDs resolves kungal gids to catalog work ids through the batch
// external-id lookup, memoized per gid. Unresolvable gids are simply absent
// from the result (a wiki entry the registrar has not reached yet, or a hard
// deleted one) — callers treat absence as "no such work", never as an error.
//
// The lookup always runs with nsfw=1: it resolves an IDENTITY, and gating it
// by the caller's content preference would make an r18 game unresolvable
// rather than merely invisible. The visibility gate belongs on the row fetch
// that follows, which is where it is applied.
func (c *GalgameClient) catalogIDsForGIDs(ctx context.Context, gids []int) (map[int]int64, *errors.AppError) {
	out := make(map[int]int64, len(gids))
	var missing []int
	now := time.Now()

	c.gidMu.RLock()
	for _, gid := range gids {
		if e, ok := c.gidCache[gid]; ok && now.Before(e.expire) {
			if e.found {
				out[gid] = e.catalogID
			}
		} else {
			missing = append(missing, gid)
		}
	}
	c.gidMu.RUnlock()
	if len(missing) == 0 {
		return out, nil
	}

	// Each gid costs one lookup item PER source key, so the gid stride is the
	// face's item ceiling divided by however many keys are in flight.
	gidStride := max(catalogIDsChunk/len(anchorSourceKeys), 1)

	resolved := make(map[int]int64, len(missing))
	for start := 0; start < len(missing); start += gidStride {
		end := min(start+gidStride, len(missing))
		chunk := missing[start:end]

		items := make([]map[string]string, 0, len(chunk)*len(anchorSourceKeys))
		for _, gid := range chunk {
			for _, source := range anchorSourceKeys {
				items = append(items, map[string]string{
					"source":      source,
					"external_id": strconv.Itoa(gid),
					"type":        "work",
				})
			}
		}
		data, appErr := c.postFace(ctx, c.v1Base, "/catalog/lookup/batch",
			url.Values{"nsfw": {"1"}}, map[string]any{"items": items}, c.apiKey)
		if appErr != nil {
			return nil, appErr
		}
		var parsed struct {
			Items []struct {
				ExternalID string `json:"external_id"`
				Work       *struct {
					ID int64 `json:"id"`
				} `json:"work"`
			} `json:"items"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, errors.ErrInternal("解析 Catalog 批量解析响应失败")
		}
		// A gid can come back once per source key. Both name source 12, so
		// both name the same registry row — whichever lands last is the same
		// identity, and only one of them resolves at all outside the rename.
		for _, it := range parsed.Items {
			if it.Work == nil {
				continue
			}
			gid, err := strconv.Atoi(it.ExternalID)
			if err != nil {
				continue
			}
			resolved[gid] = it.Work.ID
		}
	}

	// Anything the anchors did not answer gets one more chance, through the
	// identity route.
	var unresolved []int
	for _, gid := range missing {
		if _, ok := resolved[gid]; !ok {
			unresolved = append(unresolved, gid)
		}
	}
	if len(unresolved) > 0 {
		adopted, appErr := c.adoptedWorkIDs(ctx, unresolved)
		if appErr != nil {
			return nil, appErr
		}
		for gid, id := range adopted {
			resolved[gid] = id
			out[gid] = id
		}
	}

	c.gidMu.Lock()
	if len(c.gidCache) > batchCacheMaxEntries {
		clear(c.gidCache)
	}
	for _, gid := range missing {
		id, ok := resolved[gid]
		ttl := gidLookupMissTTL
		if ok {
			ttl = gidLookupHitTTL
			out[gid] = id
		}
		c.gidCache[gid] = gidLookupEntry{catalogID: id, found: ok, expire: now.Add(ttl)}
	}
	c.gidMu.Unlock()
	return out, nil
}

// adoptedWorkIDs is the second half of the gid bridge, and it exists because
// the registry now issues kungal's ids.
//
// A work minted through the submission face carries NO external_ref anchor: an
// anchor records the id some upstream issued for a work, and for a brand-new
// submission there is no upstream — the claim adopts the work's own primary
// key. So the anchor lookup above, which is the only route the bridge had,
// answers nothing for every entry submitted after the switchover, and answers
// it as "no such work" rather than as an error. Every page of a new entry would
// 404 silently.
//
// The route back is to read the id AS a work id — but only after the row agrees
// that it is the one being asked for. THE ROUND-TRIP CHECK IS NOT DEFENSIVE, IT
// IS THE WHOLE CORRECTNESS ARGUMENT: a legacy gid is also a syntactically valid
// work id, so resolving gid 42 by fetching work 42 would hand back a different
// game. An adopted id satisfies `claimed_by.work_id == the id we asked for` by
// construction; a coincidence does not.
//
// The gates are opened for the same reason the anchor lookup opens them: this
// resolves an IDENTITY. Filtering it by the reader's content preference would
// make an r18 entry unresolvable rather than merely invisible, and the
// visibility decision belongs to the row fetch that follows.
func (c *GalgameClient) adoptedWorkIDs(ctx context.Context, gids []int) (map[int]int64, *errors.AppError) {
	ids := make([]int64, len(gids))
	for i, gid := range gids {
		ids[i] = int64(gid)
	}
	rows, appErr := c.worksByCatalogIDs(ctx, ids, "", "all")
	if appErr != nil {
		return nil, appErr
	}
	out := make(map[int]int64, len(rows))
	for i := range rows {
		row := &rows[i]
		// gid() already requires the claim to be ours; this additionally
		// requires it to point back at the id we asked for.
		if gid := row.gid(); gid > 0 && int64(gid) == row.ID {
			out[gid] = row.ID
		}
	}
	return out, nil
}

// worksByCatalogIDs fetches catalog rows by catalog id, chunked at the face's
// own ceiling. limit is pinned to the chunk size: the works lane defaults to 20
// and would otherwise silently return the first 20 of a 100-id request.
func (c *GalgameClient) worksByCatalogIDs(ctx context.Context, ids []int64, include, contentLimit string) ([]CatalogWorkListItem, *errors.AppError) {
	var out []CatalogWorkListItem
	for start := 0; start < len(ids); start += catalogIDsChunk {
		end := min(start+catalogIDsChunk, len(ids))
		chunk := ids[start:end]

		raw := make([]string, len(chunk))
		for i, id := range chunk {
			raw[i] = strconv.FormatInt(id, 10)
		}
		q := url.Values{
			"ids":   {strings.Join(raw, ",")},
			"limit": {strconv.Itoa(catalogIDsChunk)},
		}
		if include != "" {
			q.Set("include", include)
		}
		applyWorksGate(q, contentLimit)

		data, appErr := c.GetV1(ctx, "/catalog/works", q)
		if appErr != nil {
			return nil, appErr
		}
		var parsed catWorksListData
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, errors.ErrInternal("解析 Catalog 作品列表响应失败")
		}
		out = append(out, parsed.Items...)
	}
	return out, nil
}

// CatalogWorkIDs exposes the identity half of the bridge on its own: gids in,
// catalog work ids out, absent for anything the registry does not know.
//
// The lifecycle and editing lanes need this WITHOUT the row fetch that
// CatalogRowsByGIDs bolts on — an action addresses a work by registry id and
// reads nothing back — and deriving the mapping from a rendered row would make
// those lanes depend on the content gates, which have no business deciding
// whether a moderator may approve something or an owner may edit it.
func (c *GalgameClient) CatalogWorkIDs(ctx context.Context, gids []int) (map[int]int64, *errors.AppError) {
	if len(gids) == 0 {
		return map[int]int64{}, nil
	}
	return c.catalogIDsForGIDs(ctx, gids)
}

// GIDsByCatalogIDs is the bridge run BACKWARDS: registry work ids in, kungal
// gids out, absent for works kungal does not claim.
//
// The editing engine keys everything on the registry id now, while every kungal
// URL is a gid — so each proposal, revision and activity row has to come home
// before it can be linked. There is no reverse memo: the forward cache is keyed
// by gid, and the rows that need this arrive in pages that are already batched.
func (c *GalgameClient) GIDsByCatalogIDs(ctx context.Context, ids []int64) (map[int64]int, *errors.AppError) {
	if len(ids) == 0 {
		return map[int64]int{}, nil
	}
	rows, appErr := c.worksByCatalogIDs(ctx, ids, "", "all")
	if appErr != nil {
		return nil, appErr
	}
	out := make(map[int64]int, len(rows))
	for i := range rows {
		// NOT filtered by isRenderable: this is an identity lookup. A withdrawn
		// entry still has edit history, and a reviewer still has to reach it.
		if gid := rows[i].gid(); gid > 0 {
			out[rows[i].ID] = gid
		}
	}
	return out, nil
}

// CatalogRowsByGIDs is the whole bridge in one call: kungal gids in, catalog
// rows out, keyed back by gid. Rows whose claim the wiki has withdrawn
// (state=hidden) are dropped here — a single choke point, so no lane can
// forget it and resurrect a banned entry.
func (c *GalgameClient) CatalogRowsByGIDs(ctx context.Context, gids []int, include, contentLimit string) (map[int]CatalogWorkListItem, *errors.AppError) {
	if len(gids) == 0 {
		return map[int]CatalogWorkListItem{}, nil
	}
	idMap, appErr := c.catalogIDsForGIDs(ctx, gids)
	if appErr != nil {
		return nil, appErr
	}
	if len(idMap) == 0 {
		return map[int]CatalogWorkListItem{}, nil
	}
	ids := make([]int64, 0, len(idMap))
	for _, id := range idMap {
		ids = append(ids, id)
	}
	rows, appErr := c.worksByCatalogIDs(ctx, ids, include, contentLimit)
	if appErr != nil {
		return nil, appErr
	}
	out := make(map[int]CatalogWorkListItem, len(rows))
	for i := range rows {
		row := rows[i]
		if !row.isRenderable() {
			continue
		}
		if gid := row.gid(); gid > 0 {
			out[gid] = row
		}
	}
	return out, nil
}

// ─── works detail ────────────────────────────────────────────────────────

// catWorkDetail is the catalog aggregate work record (GET
// /v1/catalog/works/{id}). Only the blocks kungal renders are typed.
type catWorkDetail struct {
	ID            int64         `json:"id"`
	DisplayName   string        `json:"display_name"`
	OLang         string        `json:"olang"`
	ContentRating string        `json:"content_rating"`
	ReleaseDate   *string       `json:"release_date"`
	Updated       string        `json:"updated"`
	Created       string        `json:"created"`
	ClaimedBy     *catClaimedBy `json:"claimed_by"`

	Titles []struct {
		Lang  string `json:"lang"`
		Title string `json:"title"`
		Kind  string `json:"kind"`
	} `json:"titles"`
	Refs []catRef `json:"refs"`
	Tags []struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		CanonicalID int64  `json:"canonical_id"`
		Tier        string `json:"tier"`
		Kind        string `json:"kind"`
		Spoiler     int    `json:"spoiler"`
		Sexual      bool   `json:"sexual"`
		// WorkCount is omitempty upstream — only a canonical-mapped row has a
		// count to report. An unmapped row therefore has NO key here, forever,
		// and decodes to 0; those rows are dropped from the chip strip anyway.
		WorkCount int `json:"work_count"`
	} `json:"tags"`
	Intro []struct {
		Lang    string `json:"lang"`
		Intro   string `json:"intro"`
		Machine bool   `json:"machine"`
	} `json:"intro"`
	Covers []struct {
		URL       string `json:"url"`
		Kind      string `json:"kind"`
		Sexual    int    `json:"sexual"`
		Violence  int    `json:"violence"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Thumbhash string `json:"thumbhash"`
	} `json:"covers"`
	Screenshots []struct {
		URL       string `json:"url"`
		Caption   string `json:"caption"`
		Sexual    int    `json:"sexual"`
		Violence  int    `json:"violence"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Thumbhash string `json:"thumbhash"`
	} `json:"screenshots"`
	Labels  []catWorkLabel  `json:"labels"`
	Engines []catWorkEngine `json:"engines"`
	Links   []catWorkLink   `json:"links"`
	// Series is first-class catalog_series membership — the grouping entity
	// works?series_id= filters on, and the address of the series page. NOT
	// series_siblings, which is vndb's pairwise same_series closure projected to
	// briefs: that one has no entity behind it and nothing to link to.
	Series []catWorkSeries `json:"series"`
}

// catWorkSeries is one series membership on a work record. member_count is the
// series' whole membership upstream, published on this product or not, so it is
// deliberately NOT projected: the page behind the link renders its own,
// forum-local count, and two different numbers for one link is the bug the
// taxonomy chips already learned to avoid.
type catWorkSeries struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CatalogWorkDetail fetches one work's full aggregate by kungal gid. found is
// false when the gid resolves to nothing or when the wiki has withdrawn the
// claim — never because of the caller's content preference.
//
// The detail lane carries NO content gate by design. The page it feeds is
// addressed by gid: the visitor asked for this exact entry, and kungal answers
// with its own display gate (the 确认显示 card + noindex, both keyed on the
// entry's editorial content_limit). Gating here instead 404'd every r18 game
// for an SFW visitor — a broken link, not a filtered list.
func (c *GalgameClient) CatalogWorkDetail(ctx context.Context, gid int) (*catWorkDetail, bool, *errors.AppError) {
	idMap, appErr := c.catalogIDsForGIDs(ctx, []int{gid})
	if appErr != nil {
		return nil, false, appErr
	}
	catalogID, ok := idMap[gid]
	if !ok {
		return nil, false, nil
	}
	// spoilers=0 keeps the character roster's spoiler traits closed; the tag
	// edges carry their own per-edge spoiler level for the FE to gate on.
	q := url.Values{"spoilers": {"0"}}
	openPopulation(q)
	data, appErr := c.GetV1(ctx, "/catalog/works/"+strconv.FormatInt(catalogID, 10), q)
	if appErr != nil {
		if appErr.StatusCode == 404 {
			return nil, false, nil
		}
		return nil, false, appErr
	}
	var d catWorkDetail
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, false, errors.ErrInternal("解析 Catalog 作品详情响应失败")
	}
	if d.ClaimedBy != nil && d.ClaimedBy.State == claimStateHidden {
		return nil, false, nil
	}
	return &d, true, nil
}

// ─── browse / search / calendar ──────────────────────────────────────────

// CatalogWorksPage is one page of the browse or search lane, already projected.
type CatalogWorksPage struct {
	Items      []CatalogWorkListItem
	NextCursor string
	Total      int64
	Count      int64
	Month      string
	Year       string
	Meta       catCalendarMeta
}

// CatalogWorksList runs the keyset browse lane with an arbitrary filter set.
// Used by the taxonomy member reverse-lookup (works?tag_id= / label_id= /
// engine_id=), which replaced the deprecated face's {ent}/{id}/galgame-ids.
func (c *GalgameClient) CatalogWorksList(ctx context.Context, q url.Values) (*CatalogWorksPage, *errors.AppError) {
	data, appErr := c.GetV1(ctx, "/catalog/works", q)
	if appErr != nil {
		return nil, appErr
	}
	var parsed catWorksListData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.ErrInternal("解析 Catalog 作品列表响应失败")
	}
	page := &CatalogWorksPage{Items: parsed.Items}
	if parsed.NextCursor != nil {
		page.NextCursor = *parsed.NextCursor
	}
	return page, nil
}

// CatalogMemberGIDs walks the browse lane for every work under one taxonomy
// filter and returns the kungal gids among them, in catalog id order.
//
// The entity detail pages then run kungal's OWN local filter/sort/paginate over
// these ids, exactly as they did with the deprecated galgame-ids op — so the
// page keeps working with local resource filters the catalog knows nothing
// about. Only CLAIMED works contribute: an unclaimed catalog work has no
// kungal row to filter, and no gid to key one by.
//
// The population is PUBLISHED members only (doc 106 §37, extended to the entity
// pages by R4): `claimed=true` alone admits an entry the wiki has claimed but
// never published — state=draft — which then rendered on the 词条 page as a
// 未发布 card. That is the entity-page instance of the search leak.
//
// pageCap bounds the walk; a taxonomy term with more members than that is
// truncated rather than allowed to fan out unboundedly on a request path.
func (c *GalgameClient) CatalogMemberGIDs(ctx context.Context, filter url.Values, isSFW bool, pageCap int) ([]int, *errors.AppError) {
	gids := []int{}
	cursor := ""
	for page := 0; page < pageCap; page++ {
		q := url.Values{}
		for k, v := range filter {
			q[k] = v
		}
		q.Set("limit", strconv.Itoa(catalogIDsChunk))
		// Only claimed works can map to a kungal gid, so let the face do that
		// filtering instead of paging through works we would discard. Since
		// A2-R4 this lane understands claim_state as well, and `live` is what
		// narrows the walk further to the PUBLISHED members; `claimed=true` is
		// implied by it and stays as the explicit gid requirement.
		q.Set("claimed", "true")
		q.Set("claim_state", claimStateLive)
		ApplyWorksGate(q, isSFW)
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		res, appErr := c.CatalogWorksList(ctx, q)
		if appErr != nil {
			return nil, appErr
		}
		for i := range res.Items {
			if !res.Items[i].isRenderable() {
				continue
			}
			if gid := res.Items[i].gid(); gid > 0 {
				gids = append(gids, gid)
			}
		}
		if res.NextCursor == "" {
			break
		}
		cursor = res.NextCursor
	}
	return gids, nil
}

// CatalogWorksSearch runs the product search lane (page-paginated, filters
// compiled into one upstream expression so total and items are gated
// identically — the deprecated face's content_limit trap by construction
// cannot recur here).
func (c *GalgameClient) CatalogWorksSearch(ctx context.Context, q url.Values) (*CatalogWorksPage, *errors.AppError) {
	data, appErr := c.GetV1(ctx, "/catalog/works/search", q)
	if appErr != nil {
		return nil, appErr
	}
	var parsed catWorksSearchData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.ErrInternal("解析 Catalog 搜索响应失败")
	}
	return &CatalogWorksPage{Items: parsed.Items, Total: parsed.Total}, nil
}

// CatalogCalendar fetches one calendar bucket. bucket is "" (month),
// "/pending" or "/tba".
func (c *GalgameClient) CatalogCalendar(ctx context.Context, bucket string, q url.Values) (*CatalogWorksPage, *errors.AppError) {
	data, appErr := c.GetV1(ctx, "/catalog/calendar"+bucket, q)
	if appErr != nil {
		return nil, appErr
	}
	var parsed catWorksListData
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.ErrInternal("解析 Catalog 月历响应失败")
	}
	page := &CatalogWorksPage{
		Items: parsed.Items, Count: parsed.Count,
		Month: parsed.Month, Year: parsed.Year, Meta: parsed.Meta,
	}
	if parsed.NextCursor != nil {
		page.NextCursor = *parsed.NextCursor
	}
	return page, nil
}
