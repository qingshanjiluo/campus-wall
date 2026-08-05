package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"kun-galgame-api/pkg/errors"
)

// briefCacheTTL memoizes the public batch lookups (names/covers + detail briefs)
// so the activity feed's repeated GetBatchPublic / GetBatchDetailPublic calls —
// across scroll pages and concurrent viewers — don't re-hit the galgame for the same
// galgames. Names/covers are stable enough to tolerate a couple of minutes stale.
const briefCacheTTL = 2 * time.Minute

// batchCacheMaxEntries crudely bounds each cache's memory: past it the cache is
// cleared (it re-warms within one TTL window). The feed touches recent galgames,
// so this is rarely hit.
const batchCacheMaxEntries = 4096

type batchCacheKey struct {
	id  int
	sfw bool
}

type batchCacheEntry[T any] struct {
	found  bool // false = negative cache (id absent / deleted / NSFW-filtered)
	val    T
	expire time.Time
}

// cachedBatch serves a per-(id,sfw) TTL cache over a batch fetch: returns the
// hits, calls `fetch` for ONLY the misses, then caches the results — including
// negatives, so a deleted/NSFW id isn't re-fetched on every page.
func cachedBatch[T any](
	mu *sync.RWMutex,
	cache map[batchCacheKey]batchCacheEntry[T],
	ids []int,
	sfw bool,
	fetch func([]int) (map[int]T, *errors.AppError),
) (map[int]T, *errors.AppError) {
	result := make(map[int]T, len(ids))
	var missing []int
	now := time.Now()
	mu.RLock()
	for _, id := range ids {
		if e, ok := cache[batchCacheKey{id, sfw}]; ok && now.Before(e.expire) {
			if e.found {
				result[id] = e.val
			}
		} else {
			missing = append(missing, id)
		}
	}
	mu.RUnlock()
	if len(missing) == 0 {
		return result, nil
	}
	fetched, appErr := fetch(missing)
	if appErr != nil {
		return nil, appErr
	}
	expire := now.Add(briefCacheTTL)
	mu.Lock()
	if len(cache) > batchCacheMaxEntries {
		clear(cache)
	}
	for _, id := range missing {
		v, ok := fetched[id]
		cache[batchCacheKey{id, sfw}] = batchCacheEntry[T]{found: ok, val: v, expire: expire}
		if ok {
			result[id] = v
		}
	}
	mu.Unlock()
	return result, nil
}

// GalgameClient calls the NextMoe catalog service (galgame surface) via HTTP.
//
// One face is left of what used to be three: the /internal and /api legacy
// bases retired with the wiki (wave-161 P5 deleted every route behind them;
// their last client-side callers left in wave 169). Everything this client
// still does rides the frozen public /v1 contract below.
type GalgameClient struct {
	// v1Base = {base}/v1 — the frozen public data contract; every call
	// carries the service X-API-Key.
	v1Base     string
	apiKey     string
	httpClient *http.Client
	// imageCDNBase resolves image hashes → CDN URLs inside doRequest (see
	// banner.go). Empty disables resolution (responses pass through untouched).
	imageCDNBase string

	// Public batch-lookup caches (see cachedBatch / briefCacheTTL).
	briefMu     sync.RWMutex
	briefCache  map[batchCacheKey]batchCacheEntry[GalgameBrief]
	detailMu    sync.RWMutex
	detailCache map[batchCacheKey]batchCacheEntry[GalgameDetailBrief]

	// label id → 官网 (see HydrateOfficialLinks). A work detail's labels[] rows
	// carry no links, so the maker's site is a second lookup; memoizing it keeps
	// a detail page at zero extra upstream calls for a maker it has seen.
	labelLinkMu    sync.RWMutex
	labelLinkCache map[batchCacheKey]batchCacheEntry[string]

	// canonical tag id → adult flag (see CatalogSexualTagIDs). The entity-search
	// hit shape carries no `sexual`, so the picker's SFW gate resolves it from
	// the detail lane; memoizing keeps a repeated typeahead at zero extra calls.
	tagSexualMu    sync.RWMutex
	tagSexualCache map[batchCacheKey]batchCacheEntry[bool]

	// gid → catalog work id memo (see catalogIDsForGIDs). Separate from the
	// brief caches because it has a different lifetime: a brief goes stale in
	// minutes, an identity mapping essentially never does.
	gidMu    sync.RWMutex
	gidCache map[int]gidLookupEntry
}

// New builds a galgame client for the NextMoe catalog service.
//
// baseURL is the NextMoe host base (e.g. http://catalog:9281, no /api or
// /internal suffix); the client derives {base}/internal (the internal-tier
// read face, the two cron feeds, and the user write set, all gated by
// X-API-Key) and {base}/api (the legacy staff face: taxonomy writes +
// /admin/* reads).
//
// apiKey is the internal-tier devapi key sent as X-API-Key on every
// internal-face call — reads, both feeds, and the user write set. It is
// REQUIRED: the internal face rejects keyless calls with 401 and there is no
// keyless-fallback valve any more (config load fail-fasts when a base is
// configured without a key). The user's Bearer JWT rides Authorization in
// parallel with X-API-Key on personalized reads / submissions / writes
// (dual-credential transport).
//
// imageCDNBase must match the service's KUN_IMAGE_PUBLIC_BASE_URL so
// hash-backed banners resolve to the same CDN URLs the service would build.
func New(baseURL, apiKey, imageCDNBase string) *GalgameClient {
	base := strings.TrimRight(baseURL, "/")

	// Clone the default transport and lift the per-host idle-connection
	// cap. net/http defaults MaxIdleConnsPerHost to 2, which throttles a
	// single-host service-to-service client: concurrent callers (runtime
	// list hydration, the release-date backfill's worker pool) can't reuse
	// keep-alive connections beyond 2 and pay a fresh dial per request.
	// Lifting it lets concurrent requests to the one host reuse the pool
	// instead of churning connections.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 64

	return &GalgameClient{
		v1Base: base + "/v1",
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			// Never follow a redirect. The catalog answers a merged entity id
			// with 301 + current_id precisely so the caller can redirect the
			// BROWSER in one hop; auto-following would swallow that signal and
			// return the survivor's record under the dead id — a duplicate page
			// on two URLs, which is what the 301 exists to prevent. No other
			// upstream endpoint answers 3xx, so this costs nothing elsewhere.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		imageCDNBase: imageCDNBase,
		briefCache:   map[batchCacheKey]batchCacheEntry[GalgameBrief]{},
		detailCache:  map[batchCacheKey]batchCacheEntry[GalgameDetailBrief]{},
		gidCache:     map[int]gidLookupEntry{},

		labelLinkCache: map[batchCacheKey]batchCacheEntry[string]{},
		tagSexualCache: map[batchCacheKey]batchCacheEntry[bool]{},
	}
}

// getFace performs a GET against the given base, attaching a Bearer user token
// and/or an X-API-Key when non-empty. Both may coexist (dual-credential
// transport: user JWT in Authorization, service key in X-API-Key).
func (c *GalgameClient) getFace(ctx context.Context, base, path, token string, query url.Values, apiKey string) (json.RawMessage, *errors.AppError) {
	reqURL := base + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, errors.ErrInternal("创建请求失败")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	return c.doRequest(req)
}

// postFace performs a JSON POST against the given base, attaching an
// X-API-Key when non-empty. The catalog public face needs it for exactly one
// op — the batch external-id lookup, which is a POST because its request is a
// list of (source, external_id) pairs (doc 106 Q1).
func (c *GalgameClient) postFace(ctx context.Context, base, path string, query url.Values, body any, apiKey string) (json.RawMessage, *errors.AppError) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, errors.ErrInternal("序列化请求失败")
	}
	reqURL := base + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.ErrInternal("创建请求失败")
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	return c.doRequest(req)
}

// apiResponse is the standard {code, message, data} wrapper.
type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// GetV1 performs an anonymous GET against the /v1 public data face with the
// service X-API-Key. `path` is the /v1-relative path — since the A2-3
// re-anchoring that is always a `/catalog/...` path: kungal reads the canonical
// registry face and nothing else on /v1. The deprecated `/v1/galgame` product
// face has no callers left anywhere in this repo (see catalog_face.go).
func (c *GalgameClient) GetV1(ctx context.Context, path string, query url.Values) (json.RawMessage, *errors.AppError) {
	return c.getFace(ctx, c.v1Base, path, "", query, c.apiKey)
}

// catalogMovedCode mirrors the catalog service's errors.ErrMoved (12): the
// requested ENTITY moved because its id was merged away, and the envelope's
// data carries current_id. Distinct from a 404 (never existed) — the caller
// must redirect, not render a not-found page.
const catalogMovedCode = 12

// getV1Envelope is getFace's sibling for the one case that has to see the HTTP
// status and the envelope's data TOGETHER: a merged id answers 301 with a
// non-zero code AND a data block, and doRequest deliberately collapses any
// non-zero code into an AppError, dropping the block that says where to go.
//
// The client is configured not to follow redirects (see New), so the 301
// arrives here rather than being silently replayed against the survivor's URL —
// which would hand back the survivor's record under the dead id, the exact
// outcome the catalog's 301 exists to prevent.
func (c *GalgameClient) getV1Envelope(ctx context.Context, path string, query url.Values) (int, *apiResponse, *errors.AppError) {
	reqURL := c.v1Base + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return 0, nil, errors.ErrInternal("创建请求失败")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("Galgame 服务请求失败 (传输层)",
			"method", req.Method, "url", req.URL.String(), "error", err)
		return 0, nil, errors.ErrInternal(fmt.Sprintf("Galgame 服务不可达: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, errors.ErrInternal("读取 Galgame 响应失败")
	}
	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		slog.Error("解析 Galgame 响应失败 (非 JSON 响应)",
			"method", req.Method, "url", req.URL.String(),
			"status", resp.StatusCode, "body", bodySnippet(respBody))
		return resp.StatusCode, nil, errors.New(errors.CodeBiz,
			fmt.Sprintf("Galgame 服务返回了非预期响应 (HTTP %d)", resp.StatusCode), resp.StatusCode)
	}
	result.Data = rewriteBanners(result.Data, c.imageCDNBase)
	return resp.StatusCode, &result, nil
}

// NextMoeUserGalgames is the paginated published-galgame list for a user.
type NextMoeUserGalgames struct {
	Galgames []GalgameBrief `json:"galgames"`
	Total    int64          `json:"total"`
}

// GalgameBrief is the lightweight metadata returned by /galgame/batch.
//
// Status is meaningful for Bearer-authenticated calls (which can see the
// caller's own status=3 pending / 4 declined drafts in addition to status=0).
// Anonymous calls always get status=0 entries — see 01-galgame.md.
//
// EffectiveBannerHash is the derived hash from covers[sort_order=0];
// frontend reads it (or the rewriteBanners-injected
// effective_banner_url) to render head images. banner_image_hash was
// retired in galgame PR5 (K-PR6) — no top-level field any more.
// BriefName resolves a brief's display title with the kungal fallback chain
// zh-CN → zh-TW → ja-JP → en-US, mirroring the frontend's
// getPreferredLanguageText zh-cn default. en-US is LAST on purpose: it is
// usually the VNDB romaji title, which must never win over a real Chinese or
// Japanese name. One chain lives here because both the edit face (proposal
// notices) and the interaction lane (like/favorite notices) name the same
// entries, and two copies of a preference order drift into two products.
func BriefName(b *GalgameBrief) string {
	if b == nil {
		return ""
	}
	for _, n := range []string{b.NameZhCn, b.NameZhTw, b.NameJaJp, b.NameEnUs} {
		if n != "" {
			return n
		}
	}
	return ""
}

type GalgameBrief struct {
	ID                 int    `json:"id"`
	VndbID             string `json:"vndb_id"`
	NameEnUs           string `json:"name_en_us"`
	NameJaJp           string `json:"name_ja_jp"`
	NameZhCn           string `json:"name_zh_cn"`
	NameZhTw           string `json:"name_zh_tw"`
	Banner             string `json:"banner"`
	Status             int    `json:"status"`
	ContentLimit       string `json:"content_limit"`
	UserID             int    `json:"user_id"`
	ResourceUpdateTime string `json:"resource_update_time"`
	OriginalLanguage   string `json:"original_language"`
	AgeLimit           string `json:"age_limit"`
	// U1: see NextMoeGalgameDetailFull. nil = unknown; TBA can coexist with a
	// concrete date ("predicted 2024 sometime") so don't enforce mutex.
	ReleaseDate    *string `json:"release_date"`
	ReleaseDateTBA bool    `json:"release_date_tba"`
	// U2: derived effective banner hash on briefs (galgame computes from the
	// row's covers[sort_order=0]). EffectiveBannerURL is injected by
	// rewriteBanners over the galgame response BEFORE this struct is
	// unmarshalled — declare the field so we capture it; without it
	// Go's unmarshal silently drops the walker's work and downstream
	// DTOs are stuck with only the hash. banner_image_hash retired in
	// galgame PR5 (K-PR6).
	EffectiveBannerHash      string `json:"effective_banner_hash"`
	EffectiveBannerURL       string `json:"effective_banner_url"`
	EffectiveBannerWidth     int    `json:"effective_banner_width,omitempty"`
	EffectiveBannerHeight    int    `json:"effective_banner_height,omitempty"`
	EffectiveBannerThumbhash string `json:"effective_banner_thumbhash,omitempty"`
	// Refs are the work's external identities keyed by catalog source
	// ("dlsite" → the DLsite workno verbatim, and whatever the catalog adds
	// later). Modeled as an open map rather than named fields on purpose: the
	// source registry grows, and an unknown key must be carried, not a decode
	// error. kungal reads only "dlsite" today (the 补票 purchase link).
	//
	// Sourced catalog-side by direct galgame_id lookup, so it is populated for
	// r18 and claimed works too — which is the whole feature, since 98.3% of
	// kungal's DLsite-mapped titles are r18.
	Refs map[string]string `json:"refs,omitempty"`
}

// DlsiteWorkno returns the brief's DLsite work number, or "" when absent.
func (b GalgameBrief) DlsiteWorkno() string { return b.Refs["dlsite"] }

// GalgameDetailBrief is GalgameBrief plus the introduction + officials a richer
// list view needs (the "new galgame" feed card). Served by
// GET /galgame/batch?view=detail (see docs/galgame_wiki/01-galgame.md). Officials
// are bare maker names — the caller joins them (e.g. 、) for "由 X 制作". The
// embedded GalgameBrief's release_date IS populated in the detail view (the plain
// brief omits it).
type GalgameDetailBrief struct {
	GalgameBrief
	IntroEnUS string   `json:"intro_en_us"`
	IntroJaJP string   `json:"intro_ja_jp"`
	IntroZhCN string   `json:"intro_zh_cn"`
	IntroZhTW string   `json:"intro_zh_tw"`
	Officials []string `json:"officials"`
}

// GetBatchDetailPublic is GetBatchPublic's richer sibling: it adds the intro +
// maker names a feed card renders (catalog include=intros,labels). Same NSFW
// contract as GetBatchPublic (isSFW → the catalog nsfw gate stays closed).
func (c *GalgameClient) GetBatchDetailPublic(ctx context.Context, ids []int, isSFW bool) (map[int]GalgameDetailBrief, *errors.AppError) {
	return cachedBatch(&c.detailMu, c.detailCache, ids, isSFW, func(miss []int) (map[int]GalgameDetailBrief, *errors.AppError) {
		rows, appErr := c.CatalogRowsByGIDs(ctx, miss, catalogDetailBriefInclude, contentLimitFor(isSFW))
		if appErr != nil {
			return nil, appErr
		}
		result := make(map[int]GalgameDetailBrief, len(rows))
		for gid := range rows {
			row := rows[gid]
			result[gid] = CatalogItemToDetailBrief(&row)
		}
		return result, nil
	})
}

// GetBatch fetches lightweight galgame info for multiple IDs with NO content
// gate (the permissive default: the caller already knows which ids it wants).
// Returns a map[galgameID] -> GalgameBrief for easy lookup.
//
// Any path reachable by anonymous traffic MUST use GetBatchPublic instead.
func (c *GalgameClient) GetBatch(ctx context.Context, ids []int) (map[int]GalgameBrief, *errors.AppError) {
	return c.batchByGIDs(ctx, ids, "all")
}

// GetBatchPublic is the cookie-aware batch fetch for any public list /
// feed enrichment path: enriches kungal-local IDs with galgame briefs while
// honouring the caller's NSFW preference.
//
//	isSFW=true  → content_limit=sfw  (drop NSFW server-side)
//	isSFW=false → content_limit=all  (caller opted in to NSFW)
//
// Per docs/galgame_wiki/00-handbook §16 the batch lane defaults to NO filter
// (callers presumed to know the IDs they want). Any path reachable by anonymous
// traffic / search crawlers MUST go through this helper rather than the bare
// GetBatch — see §16 "不要在下游做客户端 filtering" for why service-layer
// post-filtering isn't equivalent (the data has already left the boundary).
func (c *GalgameClient) GetBatchPublic(ctx context.Context, ids []int, isSFW bool) (map[int]GalgameBrief, *errors.AppError) {
	return cachedBatch(&c.briefMu, c.briefCache, ids, isSFW, func(miss []int) (map[int]GalgameBrief, *errors.AppError) {
		return c.batchByGIDs(ctx, miss, contentLimitFor(isSFW))
	})
}

// batchByGIDs is the shared body of the batch lane: kungal gids in, kungal
// briefs out, over the catalog works face (see catalog_face.go for the two-hop
// id bridge). Ids the registry cannot resolve, rows the content gate drops and
// rows the wiki has withdrawn are simply absent from the result — the caller's
// "not found ⇒ skip this card" branch already handles all three.
func (c *GalgameClient) batchByGIDs(ctx context.Context, ids []int, contentLimit string) (map[int]GalgameBrief, *errors.AppError) {
	if len(ids) == 0 {
		return map[int]GalgameBrief{}, nil
	}
	rows, appErr := c.CatalogRowsByGIDs(ctx, ids, catalogBriefInclude, contentLimit)
	if appErr != nil {
		return nil, appErr
	}
	result := make(map[int]GalgameBrief, len(rows))
	for gid := range rows {
		row := rows[gid]
		result[gid] = CatalogItemToBrief(&row)
	}
	return result, nil
}

// bodySnippet trims an upstream body for safe logging / error context.
// Galgame errors are tiny JSON; a misconfigured upstream may return a large
// HTML page, so cap it.
func bodySnippet(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

// The user-submission write lane retired with the wiki's submission surface.
// SubmitDraft / ClaimDraft / PatchDraft / DeleteDraft all wrote a wiki row's
// `status` column; the lifecycle is claim_state now, moved through semantic
// actions on the registry (pkg/catalogclient/claims.go), so there is nothing
// left for this client to forward.

func (c *GalgameClient) doRequest(req *http.Request) (json.RawMessage, *errors.AppError) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Transport-level failure: galgame unreachable / DNS / timeout /
		// connection refused. The most common operational cause is the
		// galgame service simply not running on the configured base URL.
		slog.Error("Galgame 服务请求失败 (传输层)",
			"method", req.Method, "url", req.URL.String(), "error", err)
		return nil, errors.ErrInternal(fmt.Sprintf("Galgame 服务不可达: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("读取 Galgame 响应失败",
			"method", req.Method, "url", req.URL.String(), "error", err)
		return nil, errors.ErrInternal("读取 Galgame 响应失败")
	}

	var result apiResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Galgame returned something that isn't the {code,message,data}
		// envelope — almost always a Fiber default error page (e.g. a
		// plain-text "Cannot POST /api/galgame/30831/claim" when the
		// running galgame binary predates the submission endpoints, or an
		// upstream proxy 5xx HTML). Surface the real HTTP status + body
		// so this is diagnosable instead of a blanket 500.
		slog.Error("解析 Galgame 响应失败 (非 JSON 响应)",
			"method", req.Method, "url", req.URL.String(),
			"status", resp.StatusCode, "body", bodySnippet(respBody))
		return nil, errors.New(
			errors.CodeBiz,
			fmt.Sprintf(
				"Galgame 服务返回了非预期响应 (HTTP %d), 请确认 galgame 服务已部署对应接口",
				resp.StatusCode,
			),
			resp.StatusCode,
		)
	}

	if result.Code != 0 {
		// Transparently forward galgame service error code + message.
		return nil, errors.New(result.Code, result.Message, resp.StatusCode)
	}

	// Resolve image_service hash-backed banners → CDN URLs once, here,
	// for EVERY galgame payload (typed mappers + verbatim passthroughs
	// like /galgame/mine). Cosmetic + fail-safe: see banner.go.
	return rewriteBanners(result.Data, c.imageCDNBase), nil
}
