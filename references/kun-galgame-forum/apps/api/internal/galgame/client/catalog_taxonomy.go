package client

// The catalog taxonomy read lanes — labels (会社/品牌), canonical tags and
// engines — plus the entity search that backs the pickers.
//
// Vocabulary note: the deprecated face called a maker an "official"; the
// catalog calls the same thing a LABEL. The kungal product wording (会社) is
// unchanged, so the rename lives only in this package and the URL space.
//
// All three list lanes are keyset (cursor) rather than page/offset, and every
// envelope now carries `total` (A2-1e), which is what lets the kungal list
// endpoints keep their {items, total} contract and their pager.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"kun-galgame-api/pkg/errors"
)

// CatalogTaxonomyItem is one row of any of the three browse lanes, flattened to
// the union of their fields. Kind/Tier are tag- and label-specific; Description
// and Aliases are engine-specific on the LIST lane (the engine facet is a few
// hundred hand-curated rows, so the face ships them inline).
type CatalogTaxonomyItem struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Kind        string   `json:"kind"`
	Tier        string   `json:"tier"`
	WorkCount   int      `json:"work_count"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	// Sexual is the tag row's own adult-content flag — tag-specific, like
	// Kind/Tier. It is the axis the SFW view gates on; false means the tag's
	// vocabulary carries NO such axis (a tag minted purely from Bangumi /
	// DLsite folksonomy), NOT an assertion that the tag is safe.
	Sexual bool `json:"sexual"`
	// HasNSFW says whether the SERIES contains any adult member — the one
	// question a grouping raises that a per-work gate cannot answer, since
	// filtering the members of a mixed series leaves a fragment rather than a
	// series. Series lane only.
	//
	// A POINTER because absent ≠ false: a catalog that predates the field
	// would otherwise report every series as clean and put r18 covers in front
	// of SFW readers. nil = unanswered, and the caller probes instead.
	HasNSFW *bool `json:"has_nsfw"`
}

// TagTierHidden is the canonical vocabulary's "do not display" tier. Upstream
// parks junk and non-browse-worthy terms there, so a hidden tag belongs in no
// browse list, no search feed and no picker — only on its own page, reached by
// a direct link.
const TagTierHidden = "hidden"

// Label returns the row's display label, papering over the two naming
// conventions (labels use display_name, tags/engines use name).
func (t CatalogTaxonomyItem) Label() string {
	if t.DisplayName != "" {
		return t.DisplayName
	}
	return t.Name
}

// CatalogTaxonomyPage is one page of a browse lane.
type CatalogTaxonomyPage struct {
	Items      []CatalogTaxonomyItem `json:"items"`
	NextCursor *string               `json:"next_cursor"`
	Total      int64                 `json:"total"`
}

// CatalogIntro is one language's description of a taxonomy entity.
type CatalogIntro struct {
	Lang   string `json:"lang"`
	Intro  string `json:"intro"`
	Source string `json:"source"`
}

// CatalogLabelDetail is the full label record.
type CatalogLabelDetail struct {
	ID          int64              `json:"id"`
	DisplayName string             `json:"display_name"`
	Kind        string             `json:"kind"`
	Lang        string             `json:"lang"`
	Aliases     []string           `json:"aliases"`
	WorkCount   int                `json:"work_count"`
	Intros      []CatalogIntro     `json:"intros"`
	Links       []CatalogLabelLink `json:"links"`
}

// CatalogLabelLink is one of a label's web presences. Source is the catalog's
// own key (official_site / twitter / cien — see PrimaryLabelLink), which is
// what a consumer needs to NAME the link rather than guess at it.
type CatalogLabelLink struct {
	Source string `json:"source"`
	URL    string `json:"url"`
}

// CatalogTagDetail is the full canonical-tag record.
type CatalogTagDetail struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Tier      string         `json:"tier"`
	Kind      string         `json:"kind"`
	WorkCount int            `json:"work_count"`
	Sexual    bool           `json:"sexual"`
	Intros    []CatalogIntro `json:"intros"`
}

// CatalogEngineDetail is the full engine record.
type CatalogEngineDetail struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	WorkCount   int      `json:"work_count"`
}

// CatalogSeriesDetail is one series record (/v1/catalog/series/{id}).
//
// The name is display_name, not name: a series carries the source's series name
// verbatim, falling back to its external id when the source published none.
// Intros are NOT merged to one row per language upstream, so the first row of
// the preferred language is the one to render.
type CatalogSeriesDetail struct {
	ID int64 `json:"id"`
	// HasNSFW — see CatalogTaxonomyItem.HasNSFW. Pointer for the same reason.
	HasNSFW     *bool  `json:"has_nsfw"`
	DisplayName string `json:"display_name"`
	Intros      []struct {
		Lang   string `json:"lang"`
		Intro  string `json:"intro"`
		Source string `json:"source"`
	} `json:"intros"`
}

// CatalogTaxonomyList pages one browse lane. entity is "labels" | "tags" |
// "engines".
func (c *GalgameClient) CatalogTaxonomyList(ctx context.Context, entity string, q url.Values) (*CatalogTaxonomyPage, *errors.AppError) {
	data, appErr := c.GetV1(ctx, "/catalog/"+entity, q)
	if appErr != nil {
		return nil, appErr
	}
	var page CatalogTaxonomyPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, errors.ErrInternal("解析 Catalog 词表响应失败")
	}
	return &page, nil
}

// CatalogTaxonomyPageAt walks the keyset lane to the requested 1-based page.
//
// The kungal list endpoints publish {items, total} with page/limit, and the
// catalog lane is cursor-based, so the offset has to be walked. That is cheap
// where it matters — these pages are browsed from the front, and the walk uses
// the face's maximum page size, so reaching kungal page N costs
// ceil(N*limit/100) upstream calls. A caller that jumps deep pays for it once;
// nothing else does.
func (c *GalgameClient) CatalogTaxonomyPageAt(ctx context.Context, entity string, base url.Values, page, limit int) ([]CatalogTaxonomyItem, int64, *errors.AppError) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	skip := (page - 1) * limit
	cursor := ""
	var total int64
	collected := make([]CatalogTaxonomyItem, 0, limit)

	for {
		q := url.Values{}
		for k, v := range base {
			q[k] = v
		}
		q.Set("limit", strconv.Itoa(catalogIDsChunk))
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		res, appErr := c.CatalogTaxonomyList(ctx, entity, q)
		if appErr != nil {
			return nil, 0, appErr
		}
		total = res.Total
		for i := range res.Items {
			if skip > 0 {
				skip--
				continue
			}
			if len(collected) < limit {
				collected = append(collected, res.Items[i])
			}
		}
		if len(collected) >= limit || res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	return collected, total, nil
}

// CatalogLabel / CatalogTag / CatalogEngine fetch one full taxonomy record.
// found=false on an unknown id (a 404, which is not an error for a page that
// wants to render its own not-found state).
//
// movedTo is non-zero on the third outcome, which only LABELS can produce:
// the id was merged away and its identity now lives on that survivor. The
// caller must 301 the browser there — never render the record it would find
// under the old id. (Tags and engines are not merge-capable upstream, so their
// movedTo is structurally always 0; it is threaded through anyway so the day
// that changes is a compile-time conversation, not a silent ghost page.)
func (c *GalgameClient) CatalogLabel(ctx context.Context, id string) (*CatalogLabelDetail, bool, int64, *errors.AppError) {
	var rec CatalogLabelDetail
	found, movedTo, appErr := c.catalogTaxonomyDetail(ctx, "labels", id, &rec)
	return &rec, found, movedTo, appErr
}

func (c *GalgameClient) CatalogTag(ctx context.Context, id string) (*CatalogTagDetail, bool, *errors.AppError) {
	var rec CatalogTagDetail
	found, _, appErr := c.catalogTaxonomyDetail(ctx, "tags", id, &rec)
	return &rec, found, appErr
}

func (c *GalgameClient) CatalogEngine(ctx context.Context, id string) (*CatalogEngineDetail, bool, *errors.AppError) {
	var rec CatalogEngineDetail
	found, _, appErr := c.catalogTaxonomyDetail(ctx, "engines", id, &rec)
	return &rec, found, appErr
}

// CatalogSeries fetches one series record. It rides the same shared detail
// helper as the other three families even though series sit on their own face:
// the envelope, the open population and the 404-is-not-an-error posture are
// identical, and series are not merge-capable upstream (no catalog_redirect
// entity type at all), so the movedTo leg is structurally dead here.
func (c *GalgameClient) CatalogSeries(ctx context.Context, id string) (*CatalogSeriesDetail, bool, *errors.AppError) {
	var rec CatalogSeriesDetail
	found, _, appErr := c.catalogTaxonomyDetail(ctx, "series", id, &rec)
	return &rec, found, appErr
}

// catalogTaxonomyDetail is the shared fetch+decode for the three detail lanes.
//
// The population runs OPEN (nsfw=1) and takes no caller preference: work_count
// here is age-aware, and an SFW caller who closed the age gate saw the count —
// and the member list behind it — collapse to the 5.5% of the registry that is
// not r18. The editorial gate has no counterpart on this face, so an SFW
// caller's count can still include the handful of entries an editor flagged;
// that is a rounding error next to the collapse, and the members lane (which
// does carry the editorial gate) remains the authority on what renders.
func (c *GalgameClient) catalogTaxonomyDetail(ctx context.Context, entity, id string, out any) (bool, int64, *errors.AppError) {
	q := openPopulation(url.Values{})
	status, env, appErr := c.getV1Envelope(ctx, "/catalog/"+entity+"/"+id, q)
	if appErr != nil {
		return false, 0, appErr
	}
	switch {
	case status == http.StatusNotFound:
		return false, 0, nil
	case status == http.StatusMovedPermanently && env.Code == catalogMovedCode:
		var moved struct {
			CurrentID int64 `json:"current_id"`
		}
		if err := json.Unmarshal(env.Data, &moved); err != nil || moved.CurrentID == 0 {
			// A 301 we cannot read is a miss, not a 500: the entity really is
			// gone from this id either way, and a broken redirect payload must
			// not take the page down with it.
			return false, 0, nil
		}
		return false, moved.CurrentID, nil
	case env.Code != 0:
		return false, 0, errors.New(env.Code, env.Message, status)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return false, 0, errors.ErrInternal("解析 Catalog 词表详情响应失败")
	}
	return true, 0, nil
}

// CatalogEntityHit is one entity-search hit.
type CatalogEntityHit struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	Name       string `json:"name"`
	// Tier / Kind ride TAG hits only (the same vocabulary the browse lane
	// renders) and are absent on every other family. Tier is what lets the tag
	// picker drop `hidden` rows without a second lookup; the hit shape carries
	// no `sexual` flag, which is why the SFW gate needs CatalogSexualTagIDs.
	Tier string `json:"tier"`
	Kind string `json:"kind"`
}

// CatalogEntitySearch runs the shared entity search. searchType is the face's
// own vocabulary: "labels" | "tags" | "names" | "characters" | "works".
//
// The hit shape is identity-only (id + name) by design — it is a picker feed.
// Consumers that need counts or metadata follow the id to the detail lane.
//
// Like every other identity lane it runs with the age gate open: a picker that
// cannot resolve an r18 maker's name is a picker that cannot be used for 94.5%
// of the registry.
func (c *GalgameClient) CatalogEntitySearch(ctx context.Context, searchType, keywords string, limit int) ([]CatalogEntityHit, *errors.AppError) {
	q := url.Values{
		"type":  {searchType},
		"q":     {keywords},
		"limit": {strconv.Itoa(limit)},
	}
	openPopulation(q)
	data, appErr := c.GetV1(ctx, "/catalog/search", q)
	if appErr != nil {
		return nil, appErr
	}
	var parsed struct {
		Items []CatalogEntityHit `json:"items"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, errors.ErrInternal("解析 Catalog 实体搜索响应失败")
	}
	if parsed.Items == nil {
		parsed.Items = []CatalogEntityHit{}
	}
	return parsed.Items, nil
}

// CatalogSexualTagIDs answers "which of these canonical tags are adult?" for
// the SFW gate on the tag PICKER feed.
//
// The browse lane ships each row's `sexual` flag inline, so it needs none of
// this. The entity-search hit shape does NOT (it is identity + tier + kind),
// and a picker that quietly offers 陵辱 to a SFW visitor is the same leak the
// browse list closed — so the flag is resolved from the detail lane, one call
// per tag, concurrently, and memoized on the shared TTL cache. A search page
// therefore costs at most `limit` upstream calls the first time it sees a set
// of tags and none after that; an NSFW caller pays nothing, because the gate
// never runs.
//
// Ids absent from the result are NOT sexual (that is what cachedBatch's
// negative caching stores). An id whose lookup FAILED is likewise absent — the
// same posture as the rest of this file's best-effort hydration: an upstream
// hiccup must not empty a picker, and the search call itself would already
// have failed if the catalog were really down.
func (c *GalgameClient) CatalogSexualTagIDs(ctx context.Context, ids []int) map[int]bool {
	flags, appErr := cachedBatch(
		&c.tagSexualMu, c.tagSexualCache, ids, false,
		func(missing []int) (map[int]bool, *errors.AppError) {
			return c.fetchSexualTagIDs(ctx, missing), nil
		},
	)
	if appErr != nil {
		return map[int]bool{}
	}
	return flags
}

// fetchSexualTagIDs resolves tag ids → sexual, concurrently, keeping only the
// TRUE ones: a map miss then reads as "not adult" both here and in the cache.
func (c *GalgameClient) fetchSexualTagIDs(ctx context.Context, ids []int) map[int]bool {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out = make(map[int]bool, len(ids))
	)
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rec, found, appErr := c.CatalogTag(ctx, strconv.Itoa(id))
			if appErr != nil || !found || !rec.Sexual {
				return
			}
			mu.Lock()
			out[id] = true
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return out
}

// LookupWikiLabel resolves a legacy wiki 会社 id (galgame_official PK) to its
// catalog label id, so the old /galgame-official/{wikiId} frontend URLs can 301
// onto the canonical /galgame/official/{catalogId} page in a single hop.
//
// Unlike tags and engines — whose migrations were frozen into a static map —
// makers resolve at RUNTIME through the registry's own external-ref index. The
// A2-0 registrar covered 100% of them, and the registry keeps that mapping
// correct across future merges, which a vendored file could not.
//
// found=false on an unregistered id (the caller 404s).
func (c *GalgameClient) LookupWikiLabel(ctx context.Context, wikiID int) (int64, bool, *errors.AppError) {
	// The singular lookup takes ONE source, so source 12's rename is walked
	// key by key rather than asked for in one shot. The keys are tried in
	// post-rename order, so once the rename has soaked this is one call again.
	for _, source := range anchorSourceKeys {
		id, found, appErr := c.lookupLabelBySource(ctx, source, wikiID)
		if appErr != nil || found {
			return id, found, appErr
		}
	}
	return 0, false, nil
}

func (c *GalgameClient) lookupLabelBySource(ctx context.Context, source string, wikiID int) (int64, bool, *errors.AppError) {
	q := url.Values{
		"source":      {source},
		"external_id": {strconv.Itoa(wikiID)},
		"type":        {"label"},
		// Identity resolution is not content: gating it would make an r18
		// maker's old URL 404 rather than redirect.
		"nsfw": {"1"},
	}
	data, appErr := c.GetV1(ctx, "/catalog/lookup", q)
	if appErr != nil {
		if appErr.StatusCode == 404 {
			return 0, false, nil
		}
		return 0, false, appErr
	}
	var parsed struct {
		Label *struct {
			ID int64 `json:"id"`
		} `json:"label"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return 0, false, errors.ErrInternal("解析 Catalog 反查响应失败")
	}
	if parsed.Label == nil {
		return 0, false, nil
	}
	return parsed.Label.ID, true, nil
}
