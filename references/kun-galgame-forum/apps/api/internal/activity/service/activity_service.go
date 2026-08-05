package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kun-galgame-api/internal/activity/dto"
	"kun-galgame-api/internal/activity/repository"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
)

// contentImageTokenRe matches a topic body's inline image token (/image/<64hex>),
// used as a feed-card cover fallback when a topic has no explicit cover_images.
var contentImageTokenRe = regexp.MustCompile(`/image/[0-9a-f]{64}`)

// solutionTopicLinkRe pulls the topic id from a MESSAGE_SOLUTION row's link
// (/topic/<id>) so the best-answer card can fetch + show the owning topic's title.
var solutionTopicLinkRe = regexp.MustCompile(`^/topic/(\d+)`)

// activityCacheTTL bounds how stale the "最新动态" feed can be. Short on purpose:
// it collapses the home page's per-render cost (the multi-source timeline query
// + galgame name-enrichment + OAuth identity round-trips) into one computation per
// window, while still surfacing new activity within seconds.
const activityCacheTTL = 30 * time.Second

const (
	// activityMaxRounds bounds serveKeyset's fetch-until-`limit`-survivors loop
	// so a pathological run of enrichment drops (NSFW-in-SFW, deleted-from-galgame)
	// can't fan out unboundedly. Each round is one keyset slice of `limit` rows +
	// a full enrichment pass (DB + galgame/OAuth), so this is the dominant per-request
	// cost multiplier — kept tight; a short page just lets the FE load one more.
	activityMaxRounds = 5
	// nextMoeBatchChunk caps ids per GetBatchPublic call — it sends every id in one
	// query param (no chunking). Matches the ≤100 batch convention.
	nextMoeBatchChunk = 100
)

type ActivityService struct {
	repo          *repository.ActivityRepository
	galgameClient *client.GalgameClient
	userClient    *userclient.Client
	rdb           *redis.Client
}

func NewActivityService(
	repo *repository.ActivityRepository,
	gc *client.GalgameClient,
	userClient *userclient.Client,
	rdb *redis.Client,
) *ActivityService {
	return &ActivityService{repo: repo, galgameClient: gc, userClient: userClient, rdb: rdb}
}

// Result is one page of activity plus the opaque keyset cursor for the next
// page ("" when there are no more rows behind it).
type Result struct {
	Items      []dto.ActivityItem `json:"items"`
	NextCursor string             `json:"next_cursor"`
}

// GetActivity returns a filtered activity feed. type "all" → the mixed
// timeline. isSFW is forwarded to galgame so NSFW galgame names never enter the
// public activity stream (docs/galgame_wiki/00-handbook §16).
func (s *ActivityService) GetActivity(ctx context.Context, typeStr, cursor string, limit int, isSFW, showNoResource bool) (*Result, *errors.AppError) {
	if typeStr == "all" {
		return s.GetTimeline(ctx, cursor, limit, isSFW, showNoResource)
	}
	if !s.repo.IsKnownType(typeStr) {
		return &Result{Items: []dto.ActivityItem{}, NextCursor: ""}, nil
	}
	cacheKey := fmt.Sprintf("activity:v2:%s:%s:%d:%t:%t", typeStr, cursor, limit, isSFW, showNoResource)
	// single-type feed excludes 资源/求助 topics (only the resource tab shows them).
	return s.cachedKeyset(ctx, cacheKey, []string{typeStr}, cursor, limit, isSFW, showNoResource, "normal")
}

// getCachedResult returns a cached activity Result, and true on a hit.
// Fail-open: redis.Nil (miss) or ANY error (redis down, bad JSON) is treated as
// a miss so the request still serves fresh data.
func (s *ActivityService) getCachedResult(ctx context.Context, key string) (*Result, bool) {
	if s.rdb == nil {
		return nil, false
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, false
	}
	return &result, true
}

// cacheResult best-effort stores the enriched result with activityCacheTTL.
// A redis/marshal failure is ignored — caching must never break the response.
func (s *ActivityService) cacheResult(ctx context.Context, key string, result *Result) {
	if s.rdb == nil || result == nil {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, key, raw, activityCacheTTL).Err()
}

// homeTabTypes maps a home-page feed tab to its activity types. The home page
// surfaces the feed as five tabs; "all" deliberately EXCLUDES galgame resources
// (they get their own 资源 tab) so the main stream isn't drowned by download
// spam. topic / galgame / resource / others partition every type; all = topic ∪
// galgame ∪ others. Keep in lock-step with the FE tab order (home/Container.vue).
var homeTabTypes = map[string][]string{
	"topic": {
		"TOPIC_CREATION", "TOPIC_REPLY_CREATION", "TOPIC_COMMENT_CREATION",
		// TOPIC_UPVOTE (the rich 推话题 card) replaces MESSAGE_UPVOTE, which
		// surfaced the same upvote via its notification row (1:1 → a duplicate).
		"TOPIC_UPVOTE", "MESSAGE_SOLUTION",
	},
	"galgame": {
		"GALGAME_CREATION", "GALGAME_EDIT", "GALGAME_PR_CREATION",
		"GALGAME_COMMENT_CREATION", "GALGAME_RATING_CREATION",
		"GALGAME_RATING_COMMENT_CREATION", "GALGAME_WEBSITE_CREATION",
		"GALGAME_WEBSITE_COMMENT_CREATION", "TOOLSET_CREATION",
		"TOOLSET_RESOURCE_CREATION", "TOOLSET_COMMENT_CREATION",
		"GALGAME_QUIZ_CREATION", "GALGAME_QUIZ_COMMENT_CREATION",
	},
	// 资源和求助 tab: newly published galgame resources + their comments + 资源/求助
	// topics (the sectionMode in FetchFeed scopes TOPIC_CREATION to
	// g-seeking/g-other/t-help here, and excludes them from every other tab).
	// Resource COMMENTS ride this tab rather than the galgame one so they stay next
	// to the resource they discuss.
	"resource": {
		"GALGAME_RESOURCE_CREATION", "GALGAME_RESOURCE_COMMENT_CREATION",
		"TOPIC_CREATION",
	},
	"others": {"TODO_CREATION", "UPDATE_LOG_CREATION"},
}

// homeTabSourceTypes resolves a tab to its type list ("all" = every non-resource
// type, i.e. topic ∪ galgame ∪ others). Unknown tab → nil.
func homeTabSourceTypes(tab string) []string {
	if tab == "all" {
		out := make([]string, 0, 18)
		out = append(out, homeTabTypes["topic"]...)
		// 全部 excludes Galgame 评论 (still shown in the dedicated Galgame tab).
		for _, t := range homeTabTypes["galgame"] {
			if t == "GALGAME_COMMENT_CREATION" {
				continue
			}
			out = append(out, t)
		}
		out = append(out, homeTabTypes["others"]...)
		return out
	}
	return homeTabTypes[tab]
}

// GetTab returns one of the home page's five tab feeds (all/topic/galgame/
// resource/others), merging only that bucket's sources. Unknown tab → empty.
func (s *ActivityService) GetTab(ctx context.Context, tab, cursor string, limit int, isSFW, showNoResource bool) (*Result, *errors.AppError) {
	types := homeTabSourceTypes(tab)
	if len(types) == 0 {
		return &Result{Items: []dto.ActivityItem{}, NextCursor: ""}, nil
	}
	// resource tab keeps ONLY 资源/求助 topics; every other built-in tab excludes them.
	sectionMode := "normal"
	if tab == "resource" {
		sectionMode = "help"
	}
	cacheKey := fmt.Sprintf("activity:v2:tab:%s:%s:%d:%t:%t", tab, cursor, limit, isSFW, showNoResource)
	return s.cachedKeyset(ctx, cacheKey, types, cursor, limit, isSFW, showNoResource, sectionMode)
}

// GetFeedByTypes serves a user-configured tab: `kinds` is that tab's selected
// activity kinds (the FE catalog). The topic pseudo-kinds TOPIC_NORMAL /
// TOPIC_RESOURCE_HELP collapse to TOPIC_CREATION + a section mode; unknown kinds
// are dropped. No selectable kind → empty feed.
func (s *ActivityService) GetFeedByTypes(ctx context.Context, kinds []string, cursor string, limit int, isSFW, showNoResource bool) (*Result, *errors.AppError) {
	types, sectionMode := s.resolveKinds(kinds)
	if len(types) == 0 {
		return &Result{Items: []dto.ActivityItem{}, NextCursor: ""}, nil
	}
	// A topics-ONLY tab (话题 / 资源和求助) is an entity list, not an event stream:
	// serve it from the topic table sorted by status_update_time (last activity),
	// not from feed_activity sorted by the event time.
	if len(types) == 1 && types[0] == "TOPIC_CREATION" {
		cacheKey := fmt.Sprintf("activity:v2:topicfeed:%s:%s:%d:%t", sectionMode, cursor, limit, isSFW)
		return s.cachedTopicKeyset(ctx, cacheKey, cursor, limit, isSFW, sectionMode)
	}
	cacheKey := fmt.Sprintf("activity:v2:custom:%s|%s:%s:%d:%t:%t",
		strings.Join(types, ","), sectionMode, cursor, limit, isSFW, showNoResource)
	return s.cachedKeyset(ctx, cacheKey, types, cursor, limit, isSFW, showNoResource, sectionMode)
}

// resolveKinds maps the FE kind catalog → real feed_activity types + a topic
// section mode. TOPIC_NORMAL / TOPIC_RESOURCE_HELP both resolve to TOPIC_CREATION;
// the section mode is "normal" (普通 only), "help" (资源/求助 only), or "all" (both
// selected, or no topic kind). Unknown / duplicate kinds are dropped.
func (s *ActivityService) resolveKinds(kinds []string) (types []string, sectionMode string) {
	seen := map[string]bool{}
	addType := func(t string) {
		if !seen[t] {
			types = append(types, t)
			seen[t] = true
		}
	}
	wantNormal, wantHelp := false, false
	for _, k := range kinds {
		switch k {
		case "TOPIC_NORMAL":
			wantNormal = true
			addType("TOPIC_CREATION")
		case "TOPIC_RESOURCE_HELP":
			wantHelp = true
			addType("TOPIC_CREATION")
		default:
			if s.repo.IsKnownType(k) {
				addType(k)
			}
		}
	}
	switch {
	case wantHelp && !wantNormal:
		sectionMode = "help"
	case wantNormal && !wantHelp:
		sectionMode = "normal"
	default:
		sectionMode = "all"
	}
	return types, sectionMode
}

// GetTimeline returns the mixed activity timeline across all sources.
func (s *ActivityService) GetTimeline(ctx context.Context, cursor string, limit int, isSFW, showNoResource bool) (*Result, *errors.AppError) {
	cacheKey := fmt.Sprintf("activity:v2:all:%s:%d:%t:%t", cursor, limit, isSFW, showNoResource)
	// nil types → no type filter (every feed_activity row); "normal" → the mixed
	// timeline excludes 资源/求助 topics (they live in the resource tab).
	return s.cachedKeyset(ctx, cacheKey, nil, cursor, limit, isSFW, showNoResource, "normal")
}

// cachedKeyset wraps serveKeyset with the activityCacheTTL cache-aside. Keyed by
// cursor (the page), isSFW (which galgame names the galgame returns) and
// showNoResource (whether resource-less GALGAME_CREATION rows are dropped) so
// viewers with different filters never share an entry.
func (s *ActivityService) cachedKeyset(ctx context.Context, cacheKey string, types []string, cursor string, limit int, isSFW, showNoResource bool, sectionMode string) (*Result, *errors.AppError) {
	if cached, ok := s.getCachedResult(ctx, cacheKey); ok {
		return cached, nil
	}
	result, appErr := s.serveKeyset(ctx, types, cursor, limit, isSFW, showNoResource, sectionMode)
	if appErr != nil {
		return nil, appErr
	}
	s.cacheResult(ctx, cacheKey, result)
	return result, nil
}

// serveKeyset fills one page to `limit` survivors despite enrichment drops. It
// seeks the deterministic keyset (created, type_str, id) from `cursor`; because
// enrichment can drop rows (NSFW-in-SFW, deleted-from-galgame), it keeps fetching
// the next slice — advancing the cursor past each fully-consumed batch — until
// it has `limit` survivors or the feed is exhausted. nextCursor is the LAST
// survivor's keyset, so the next page resumes exactly where this one stopped:
// no offset drift, and the total-order tiebreaker means no dup/skip across pages.
func (s *ActivityService) serveKeyset(ctx context.Context, types []string, cursor string, limit int, isSFW, showNoResource bool, sectionMode string) (*Result, *errors.AppError) {
	cur := decodeCursor(cursor)
	collected := make([]dto.ActivityItem, 0, limit)
	exhausted := false

	for round := 0; len(collected) < limit && round < activityMaxRounds; round++ {
		rows, err := s.repo.FetchFeed(types, limit, cur, isSFW, showNoResource, sectionMode)
		if err != nil {
			return nil, errors.ErrInternal("查询活动数据失败")
		}
		if len(rows) == 0 {
			exhausted = true
			break
		}
		for _, it := range s.enrichAndHydrate(ctx, rows, isSFW) {
			collected = append(collected, it)
			if len(collected) == limit {
				break
			}
		}
		if len(collected) >= limit {
			break // page full — nextCursor is the last collected survivor
		}
		// Whole batch consumed without filling the page; advance past its last
		// RAW row (some were dropped by enrichment) and fetch the next slice.
		last := rows[len(rows)-1]
		cur = &repository.Cursor{Created: last.Created, TypeStr: last.TypeStr, ID: last.ID}
		if len(rows) < limit {
			exhausted = true
			break // short batch → nothing behind it
		}
	}

	next := ""
	switch {
	case len(collected) == limit:
		// Full page — resume after the last survivor (one extra empty fetch if
		// this happened to be the very tail; harmless).
		last := collected[len(collected)-1]
		next = encodeCursor(last.Timestamp, last.Type, last.ID)
	case !exhausted && cur != nil:
		// Hit activityMaxRounds before filling the page (heavy enrichment drops):
		// resume from where we stopped rather than signalling a false end-of-feed.
		next = encodeCursor(cur.Created, cur.TypeStr, cur.ID)
	}
	return &Result{Items: collected, NextCursor: next}, nil
}

// cachedTopicKeyset / serveTopicKeyset are the 话题-tab counterparts of
// cachedKeyset / serveKeyset: the page comes from FetchTopicFeed (topics sorted by
// status_update_time) instead of FetchFeed, but the enrichment + opaque cursor are
// shared. There's no fill-to-limit loop — one fetch is one page — so a page can
// come back short when a banned author's topics are dropped by hydrateActors;
// the cursor is the last raw row, so pagination still self-heals across pages.
func (s *ActivityService) cachedTopicKeyset(ctx context.Context, cacheKey, cursor string, limit int, isSFW bool, sectionMode string) (*Result, *errors.AppError) {
	if cached, ok := s.getCachedResult(ctx, cacheKey); ok {
		return cached, nil
	}
	result, appErr := s.serveTopicKeyset(ctx, cursor, limit, isSFW, sectionMode)
	if appErr != nil {
		return nil, appErr
	}
	s.cacheResult(ctx, cacheKey, result)
	return result, nil
}

func (s *ActivityService) serveTopicKeyset(ctx context.Context, cursor string, limit int, isSFW bool, sectionMode string) (*Result, *errors.AppError) {
	rows, err := s.repo.FetchTopicFeed(limit, decodeCursor(cursor), isSFW, sectionMode)
	if err != nil {
		return nil, errors.ErrInternal("查询话题失败")
	}
	items := s.enrichAndHydrate(ctx, rows, isSFW)
	next := ""
	if len(rows) == limit {
		// Resume after the last row's keyset (status_update_time, type, id).
		last := rows[len(rows)-1]
		next = encodeCursor(last.Created, last.TypeStr, last.ID)
	}
	return &Result{Items: items, NextCursor: next}, nil
}

// ──────────────────────────────────────────
// Keyset cursor codec
// ──────────────────────────────────────────

// cursorPayload is the JSON behind the opaque base64 cursor: the last row's
// (created, type_str, id). type_str is required because `id` is unique only
// within a source — the feed UNIONs many tables.
type cursorPayload struct {
	C time.Time `json:"c"`
	T string    `json:"t"`
	I int       `json:"i"`
}

func encodeCursor(created time.Time, typeStr string, id int) string {
	b, err := json.Marshal(cursorPayload{C: created, T: typeStr, I: id})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor parses an opaque cursor into a repository.Cursor. Empty → nil
// (first page). A malformed cursor also → nil: a stale/garbage token just
// restarts from the newest rather than erroring the whole feed.
func decodeCursor(cursor string) *repository.Cursor {
	if cursor == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil
	}
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.T == "" || p.I <= 0 {
		return nil
	}
	return &repository.Cursor{Created: p.C, TypeStr: p.T, ID: p.I}
}

// enrichAndHydrate runs the full row → item pipeline: galgame name/brief
// enrichment (which drops brief-missing rows), topic rich-card enrichment, then
// OAuth identity hydration.
func (s *ActivityService) enrichAndHydrate(ctx context.Context, rows []repository.ActivityRow, isSFW bool) []dto.ActivityItem {
	items := rowsToItems(rows)
	items = s.enrichGalgameItems(ctx, rows, items, isSFW)
	s.enrichGalgameResourceDetails(items)
	s.enrichTopicItems(ctx, items)
	s.enrichTopicCommentItems(ctx, items)
	s.enrichReplyItems(ctx, items)
	s.enrichNoteItems(items)
	s.enrichQuizItems(items)
	s.enrichEntityRefItems(items)
	s.enrichSolutionItems(items)
	s.renderMarkdownBodies(items)
	items = s.hydrateActors(ctx, items)
	return items
}

// enrichSolutionItems attaches the owning topic's title to MESSAGE_SOLUTION rows
// (a best answer was accepted), so the feed card can name the topic + link to it.
// The topic id is parsed from the row's link (/topic/:id). Best-effort.
func (s *ActivityService) enrichSolutionItems(items []dto.ActivityItem) {
	topicIDByIdx := map[int]int{}
	idSet := map[int]struct{}{}
	for i, it := range items {
		if it.Type != "MESSAGE_SOLUTION" {
			continue
		}
		m := solutionTopicLinkRe.FindStringSubmatch(it.Link)
		if m == nil {
			continue
		}
		tid, _ := strconv.Atoi(m[1])
		if tid > 0 {
			topicIDByIdx[i] = tid
			idSet[tid] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	titles, err := s.repo.FetchTopicTitles(ids)
	if err != nil {
		return
	}
	// The accepted reply's floor lets the card deep-link to it (?reply=<floor>).
	// FetchTopicBestAnswers is keyed by topic id and now carries the reply's floor;
	// a topic with no current best answer falls back to Floor 0 (the topic root).
	bestAnswers, _ := s.repo.FetchTopicBestAnswers(ids)
	for i, tid := range topicIDByIdx {
		if title, ok := titles[tid]; ok {
			items[i].Data = dto.SolutionActivityData{TopicTitle: title, Floor: bestAnswers[tid].Floor}
		}
	}
}

// enrichEntityRefItems attaches the parent entity name to the toolset resource
// card (TOOLSET_RESOURCE_CREATION), whose Content is a resource note: the owning
// toolset's name. The creation cards carry the name in Content directly, so they
// need no enrichment. The toolset / website COMMENT cards no longer get a parent
// name — their comments moved onto the community primitive (charter step 06a) and
// the historical frozen-table enrichment was retired (accepted loss). Best-effort.
func (s *ActivityService) enrichEntityRefItems(items []dto.ActivityItem) {
	var resIDs []int
	for _, it := range items {
		if it.Type == "TOOLSET_RESOURCE_CREATION" {
			resIDs = append(resIDs, it.ID)
		}
	}
	if len(resIDs) == 0 {
		return
	}
	resParents, _ := s.repo.FetchToolsetResourceParents(resIDs)
	for i := range items {
		if items[i].Type == "TOOLSET_RESOURCE_CREATION" {
			if name := resParents[items[i].ID]; name != "" {
				items[i].Data = dto.EntityRefActivityData{ParentName: name}
			}
		}
	}
}

// renderMarkdownBodies renders the FULL reply / galgame-comment body to HTML, so
// the feed shows it as rich, untruncated Markdown (the same goldmark renderer the
// detail pages use). Runs after enrichReplyItems, whose @/# token resolution has
// already left Content as plain markdown text.
func (s *ActivityService) renderMarkdownBodies(items []dto.ActivityItem) {
	for i := range items {
		switch items[i].Type {
		case "TOPIC_REPLY_CREATION", "GALGAME_COMMENT_CREATION":
			if items[i].Content != "" {
				items[i].Content = markdown.Render(items[i].Content)
			}
		}
	}
}

// enrichNoteItems attaches the small extras the 其他-tab Note card shows: the
// changelog Version (UPDATE_LOG_CREATION) and the todo completion Status
// (TODO_CREATION). Best-effort — a failed lookup just leaves the badge off.
func (s *ActivityService) enrichNoteItems(items []dto.ActivityItem) {
	todoIdx := map[int][]int{}
	logIdx := map[int][]int{}
	for i, it := range items {
		switch it.Type {
		case "TODO_CREATION":
			todoIdx[it.ID] = append(todoIdx[it.ID], i)
		case "UPDATE_LOG_CREATION":
			logIdx[it.ID] = append(logIdx[it.ID], i)
		}
	}
	if len(todoIdx) > 0 {
		ids := make([]int, 0, len(todoIdx))
		for id := range todoIdx {
			ids = append(ids, id)
		}
		if m, err := s.repo.FetchTodoStatuses(ids); err == nil {
			for id, st := range m {
				status := st
				for _, i := range todoIdx[id] {
					items[i].Data = dto.NoteActivityData{Status: &status}
				}
			}
		}
	}
	if len(logIdx) > 0 {
		ids := make([]int, 0, len(logIdx))
		for id := range logIdx {
			ids = append(ids, id)
		}
		if m, err := s.repo.FetchUpdateLogVersions(ids); err == nil {
			for id, v := range m {
				for _, i := range logIdx[id] {
					items[i].Data = dto.NoteActivityData{Version: v}
				}
			}
		}
	}
}

// enrichQuizItems attaches the 出题 card payload (category / type / difficulty +
// answer stats) to GALGAME_QUIZ_CREATION rows. Item.ID is the quiz id. Best-effort:
// a query error just leaves nil Data → the generic card.
func (s *ActivityService) enrichQuizItems(items []dto.ActivityItem) {
	idToIdx := map[int][]int{}
	for i, it := range items {
		if it.Type == "GALGAME_QUIZ_CREATION" {
			idToIdx[it.ID] = append(idToIdx[it.ID], i)
		}
	}
	if len(idToIdx) == 0 {
		return
	}
	ids := make([]int, 0, len(idToIdx))
	for id := range idToIdx {
		ids = append(ids, id)
	}
	metaMap, err := s.repo.FetchQuizActivityData(ids)
	if err != nil {
		return
	}
	for id, idxs := range idToIdx {
		m, ok := metaMap[id]
		if !ok {
			continue
		}
		payload := dto.QuizActivityData{
			Category:      m.Category,
			Type:          m.Type,
			Difficulty:    m.Difficulty,
			AnswerCount:   m.AnswerCount,
			CorrectCount:  m.CorrectCount,
			FavoriteCount: m.FavoriteCount,
			Description:   m.Description,
		}
		for _, i := range idxs {
			items[i].Data = payload
		}
	}
}

// enrichTopicCommentItems attaches the owning topic title + the reply being
// commented on (被评论的评论) to TOPIC_COMMENT_CREATION rows, so the card renders
// like the reply card. The @/# tokens in both the comment body and the quoted
// reply are resolved to readable text (same as the reply card).
func (s *ActivityService) enrichTopicCommentItems(ctx context.Context, items []dto.ActivityItem) {
	idToIdx := map[int][]int{}
	for i, it := range items {
		if it.Type == "TOPIC_COMMENT_CREATION" {
			idToIdx[it.ID] = append(idToIdx[it.ID], i)
		}
	}
	if len(idToIdx) == 0 {
		return
	}
	ids := make([]int, 0, len(idToIdx))
	for id := range idToIdx {
		ids = append(ids, id)
	}
	ctxMap, err := s.repo.FetchTopicCommentContext(ids)
	if err != nil {
		return
	}

	// Resolve @-mentions across both the comment bodies and the quoted replies.
	mentionSet := map[int]struct{}{}
	for id, idxs := range idToIdx {
		for _, mid := range collectReplyMentionIDs(items[idxs[0]].Content) {
			mentionSet[mid] = struct{}{}
		}
		if c, ok := ctxMap[id]; ok {
			for _, mid := range collectReplyMentionIDs(c.ReplyContent) {
				mentionSet[mid] = struct{}{}
			}
		}
	}
	names := map[int]string{}
	if len(mentionSet) > 0 {
		mids := make([]int, 0, len(mentionSet))
		for mid := range mentionSet {
			mids = append(mids, mid)
		}
		for id, u := range s.userClient.Hydrate(ctx, mids) {
			names[id] = u.Name
		}
	}

	for id, idxs := range idToIdx {
		c, ok := ctxMap[id]
		if !ok {
			continue
		}
		payload := dto.TopicCommentActivityData{
			TopicTitle: c.TopicTitle,
			CommentId:  id,
			QuotedReply: &dto.QuotedReply{
				Floor:   c.ReplyFloor,
				Content: renderReplyTokens(c.ReplyContent, names),
			},
		}
		for _, i := range idxs {
			items[i].Content = renderReplyTokens(items[i].Content, names)
			items[i].Data = payload
		}
	}
}

// enrichGalgameResourceDetails attaches the resource spec (no download link /
// codes) to GALGAME_RESOURCE_CREATION rows, patching the galgame payload.
func (s *ActivityService) enrichGalgameResourceDetails(items []dto.ActivityItem) {
	ids := []int{}
	for _, it := range items {
		if it.Type == "GALGAME_RESOURCE_CREATION" {
			ids = append(ids, it.ID)
		}
	}
	details, err := s.repo.FetchGalgameResourceDetails(ids)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].Type != "GALGAME_RESOURCE_CREATION" {
			continue
		}
		d, ok := details[items[i].ID]
		if !ok {
			continue
		}
		if ga, ok := items[i].Data.(dto.GalgameActivityData); ok {
			ga.Resource = &dto.GalgameResourceDetails{
				Type:      d.Type,
				Language:  d.Language,
				Platform:  d.Platform,
				Size:      d.Size,
				Note:      d.Note,
				LikeCount: d.LikeCount,
			}
			items[i].Data = ga
		}
	}
}

// Reply content stores @-mentions and #-quotes as markdown links:
//
//	[@<name>](kungal-user:<id>)   and   [#<floor>](kungal-reply:<id>)
//
// For the feed we render them to readable plain text: a mention → "@<current
// name>" (resolved via OAuth), a quote → its "#<floor>" label kept as-is.
var (
	replyMentionRe = regexp.MustCompile(`\[[^\]]*\]\(kungal-user:(\d+)\)`)
	replyQuoteRe   = regexp.MustCompile(`\[(#[^\]]*)\]\(kungal-reply:(\d+)\)`)
)

func collectReplyMentionIDs(content string) []int {
	matches := replyMentionRe.FindAllStringSubmatch(content, -1)
	ids := make([]int, 0, len(matches))
	for _, m := range matches {
		if id, err := strconv.Atoi(m[1]); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// firstQuotedReplyID returns the reply id of the first #-quote token, or 0.
func firstQuotedReplyID(content string) int {
	m := replyQuoteRe.FindStringSubmatch(content)
	if m == nil {
		return 0
	}
	id, _ := strconv.Atoi(m[2])
	return id
}

// renderReplyTokens rewrites @/# tokens to readable text: mentions → "@<name>"
// (from the resolved names map; unknown → "@用户"), quotes → their "#<floor>" label.
func renderReplyTokens(content string, names map[int]string) string {
	content = replyMentionRe.ReplaceAllStringFunc(content, func(tok string) string {
		m := replyMentionRe.FindStringSubmatch(tok)
		id, _ := strconv.Atoi(m[1])
		if name := names[id]; name != "" {
			return "@" + name
		}
		return "@用户"
	})
	return replyQuoteRe.ReplaceAllString(content, "$1")
}

// enrichReplyItems builds each TOPIC_REPLY_CREATION item's reply card: it resolves
// the @/# tokens in the reply body (in place on Content), attaches the parent
// topic title, and — when the reply quoted another reply — the quoted reply's
// floor + body. Item.ID is the reply id. Best-effort: any sub-query error simply
// leaves that piece out rather than dropping the item.
func (s *ActivityService) enrichReplyItems(ctx context.Context, items []dto.ActivityItem) {
	idToIdx := map[int][]int{}
	for i, it := range items {
		if it.Type == "TOPIC_REPLY_CREATION" {
			idToIdx[it.ID] = append(idToIdx[it.ID], i)
		}
	}
	if len(idToIdx) == 0 {
		return
	}
	ids := make([]int, 0, len(idToIdx))
	for id := range idToIdx {
		ids = append(ids, id)
	}
	titles, _ := s.repo.FetchReplyTopicTitles(ids)
	// Each reply's own floor → deep-link the card to it (?reply=<floor>).
	floors, _ := s.repo.FetchReplyFloors(ids)

	// First quoted reply per reply → batch-fetch the quoted bodies.
	quotedIDByReply := map[int]int{}
	quotedIDSet := map[int]struct{}{}
	for id, idxs := range idToIdx {
		if qid := firstQuotedReplyID(items[idxs[0]].Content); qid > 0 {
			quotedIDByReply[id] = qid
			quotedIDSet[qid] = struct{}{}
		}
	}
	quotedIDs := make([]int, 0, len(quotedIDSet))
	for qid := range quotedIDSet {
		quotedIDs = append(quotedIDs, qid)
	}
	quotedContents, _ := s.repo.FetchReplyContents(quotedIDs)

	// Mention ids across both the reply bodies and the quoted bodies → resolve
	// once to current names.
	mentionSet := map[int]struct{}{}
	for _, idxs := range idToIdx {
		for _, mid := range collectReplyMentionIDs(items[idxs[0]].Content) {
			mentionSet[mid] = struct{}{}
		}
	}
	for _, qc := range quotedContents {
		for _, mid := range collectReplyMentionIDs(qc.Content) {
			mentionSet[mid] = struct{}{}
		}
	}
	names := map[int]string{}
	if len(mentionSet) > 0 {
		mids := make([]int, 0, len(mentionSet))
		for mid := range mentionSet {
			mids = append(mids, mid)
		}
		for id, u := range s.userClient.Hydrate(ctx, mids) {
			names[id] = u.Name
		}
	}

	for id, idxs := range idToIdx {
		var quoted *dto.QuotedReply
		if qc, ok := quotedContents[quotedIDByReply[id]]; ok {
			quoted = &dto.QuotedReply{
				Floor:   qc.Floor,
				Content: renderReplyTokens(qc.Content, names),
			}
		}
		data := dto.ReplyActivityData{TopicTitle: titles[id], Floor: floors[id], QuotedReply: quoted}
		for _, i := range idxs {
			items[i].Content = renderReplyTokens(items[i].Content, names)
			items[i].Data = data
		}
	}
}

// enrichTopicItems attaches the rich-card payload (covers + counts) to every
// TOPIC_CREATION item via one batch query. Item.ID is the topic id for these
// rows. Best-effort: on a query error the items keep a nil Data and fall back to
// the generic card. Runs after enrichGalgameItems (topic items are never dropped
// there — galgame_id is 0), so it operates on the surviving items.
func (s *ActivityService) enrichTopicItems(ctx context.Context, items []dto.ActivityItem) {
	idToIdx := map[int][]int{}
	for i, it := range items {
		if it.Type == "TOPIC_CREATION" {
			idToIdx[it.ID] = append(idToIdx[it.ID], i)
		}
	}
	// 推话题 (TOPIC_UPVOTE) cards reuse this enrichment: resolve each upvote row
	// id → its topic id, then map that topic id to the upvote item(s) too. The
	// upvote card reads payload.Title (its Content carries the push description).
	upvoteIdx := map[int][]int{}
	for i, it := range items {
		if it.Type == "TOPIC_UPVOTE" {
			upvoteIdx[it.ID] = append(upvoteIdx[it.ID], i)
		}
	}
	if len(upvoteIdx) > 0 {
		upvoteIDs := make([]int, 0, len(upvoteIdx))
		for id := range upvoteIdx {
			upvoteIDs = append(upvoteIDs, id)
		}
		if topicByUpvote, err := s.repo.FetchUpvoteTopics(upvoteIDs); err == nil {
			for upvoteID, idxs := range upvoteIdx {
				if tid := topicByUpvote[upvoteID]; tid > 0 {
					idToIdx[tid] = append(idToIdx[tid], idxs...)
				}
			}
		}
	}
	if len(idToIdx) == 0 {
		return
	}
	ids := make([]int, 0, len(idToIdx))
	for id := range idToIdx {
		ids = append(ids, id)
	}
	core, err := s.repo.FetchTopicActivityData(ids)
	if err != nil {
		return // graceful: fall back to the generic card
	}
	// The rest are best-effort: a failed side-load just omits that facet.
	sections, _ := s.repo.FetchTopicSections(ids)
	polls, _ := s.repo.FetchTopicPolls(ids)
	topReplies, _ := s.repo.FetchTopicTopReply(ids)
	bestAnswers, _ := s.repo.FetchTopicBestAnswers(ids)
	upvotes, _ := s.repo.FetchTopicUpvotesBatch(ids)
	latest, _ := s.repo.FetchTopicLatestActivity(ids)
	reactionRows, _ := s.repo.FetchTopicsReactions(ids)

	// Hydrate the "other" users shown on the card — top-reply authors AND the
	// reaction avatars (both shared, not per-viewer) — in one batch.
	extraIDs := make([]int, 0, len(topReplies)+len(reactionRows))
	for _, tr := range topReplies {
		if tr.UserID > 0 {
			extraIDs = append(extraIDs, tr.UserID)
		}
	}
	for _, ba := range bestAnswers {
		if ba.UserID > 0 {
			extraIDs = append(extraIDs, ba.UserID)
		}
	}
	for _, ups := range upvotes {
		for _, up := range ups {
			if up.UserID > 0 {
				extraIDs = append(extraIDs, up.UserID)
			}
		}
	}
	for _, la := range latest {
		if la.UserID > 0 {
			extraIDs = append(extraIDs, la.UserID)
		}
	}
	for _, row := range reactionRows {
		if row.UserID > 0 {
			extraIDs = append(extraIDs, row.UserID)
		}
	}
	extraUsers := s.userClient.Hydrate(ctx, extraIDs)

	// Group the windowed reaction rows per topic → one entry per key (total count
	// + up to a few reactor avatars), preserving the per-topic order.
	type rkey struct {
		tid int
		r   string
	}
	racc := map[rkey]*dto.TopicReactionCount{}
	rorder := map[int][]rkey{}
	for _, row := range reactionRows {
		k := rkey{row.TopicID, row.Reaction}
		rc, ok := racc[k]
		if !ok {
			rc = &dto.TopicReactionCount{Reaction: row.Reaction, Count: row.Count}
			racc[k] = rc
			rorder[row.TopicID] = append(rorder[row.TopicID], k)
		}
		if u, ok := extraUsers[row.UserID]; ok {
			rc.Reactors = append(rc.Reactors,
				dto.Actor{ID: u.ID, Name: u.Name, Avatar: u.Avatar})
		}
	}
	reactionsByTopic := map[int][]dto.TopicReactionCount{}
	for tid, keys := range rorder {
		for _, k := range keys {
			reactionsByTopic[tid] = append(reactionsByTopic[tid], *racc[k])
		}
	}

	for id, idxs := range idToIdx {
		c, ok := core[id]
		if !ok {
			continue
		}
		covers := []string(c.CoverImages)
		if covers == nil {
			covers = []string{}
		}
		// No explicit cover? Fall back to the first inline content image so an
		// image-only post still shows its picture in the feed instead of blank.
		if len(covers) == 0 {
			if img := contentImageTokenRe.FindString(c.Excerpt); img != "" {
				covers = []string{img}
			}
		}
		reactions := reactionsByTopic[id]
		if reactions == nil {
			reactions = []dto.TopicReactionCount{}
		}
		sec := sections[id]
		if sec == nil {
			sec = []string{}
		}
		var topReply *dto.TopReply
		if tr, ok := topReplies[id]; ok {
			topReply = &dto.TopReply{ReplyID: tr.ID, Floor: tr.Floor, Content: tr.Content, LikeCount: tr.LikeCount}
			if u, ok := extraUsers[tr.UserID]; ok {
				topReply.User = dto.Actor{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
			}
		}
		var bestAnswer *dto.TopReply
		if ba, ok := bestAnswers[id]; ok {
			bestAnswer = &dto.TopReply{ReplyID: ba.ReplyID, Floor: ba.Floor, Content: ba.Content, LikeCount: ba.LikeCount}
			if u, ok := extraUsers[ba.UserID]; ok {
				bestAnswer.User = dto.Actor{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
			}
		}
		var upvoteList []dto.TopicUpvote
		for _, up := range upvotes[id] {
			tu := dto.TopicUpvote{ID: up.ID, Description: up.Description, Created: up.Created}
			if u, ok := extraUsers[up.UserID]; ok {
				tu.User = dto.Actor{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
			}
			upvoteList = append(upvoteList, tu)
		}
		var latestActivity *dto.LatestActivity
		if la, ok := latest[id]; ok {
			latestActivity = &dto.LatestActivity{Kind: la.Kind, Content: la.Content, Created: la.Created}
			if la.Kind == "reply" {
				latestActivity.ReplyID = la.ID
				latestActivity.Floor = la.Floor
			} else {
				latestActivity.CommentId = la.ID
			}
			if u, ok := extraUsers[la.UserID]; ok {
				latestActivity.User = dto.Actor{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
			}
		}
		payload := dto.TopicActivityData{
			TopicID:     id,
			Title:       c.Title,
			AuthorID:    c.AuthorID,
			Excerpt:     c.Excerpt,
			Sections:    sec,
			CoverImages: covers,
			// Reserve each cover's aspect ratio (no CLS) + blur-up on the FE.
			CoverImageMeta: markdown.ResolveContentImageMeta(covers),
			View:           c.View,
			LikeCount:      c.LikeCount,
			FavoriteCount:  c.FavoriteCount,
			ReplyCount:     c.ReplyCount,
			CommentCount:   c.CommentCount,
			UpvoteTime:     c.UpvoteTime,
			Edited:         c.Edited,
			HasBestAnswer:  c.BestAnswerID != nil,
			IsPoll:         polls[id],
			IsNSFW:         c.IsNSFW,
			TopReply:       topReply,
			BestAnswer:     bestAnswer,
			Upvotes:        upvoteList,
			LatestActivity: latestActivity,
			Reactions:      reactions,
		}
		for _, i := range idxs {
			items[i].Data = payload
		}
	}
}

// rowsToItems converts DB rows into response items (no enrichment yet).
// Identity is left blank — hydrated by hydrateActors after enrichGalgameItems
// has had a chance to inject GALGAME_CREATION actor IDs from the galgame brief.
func rowsToItems(rows []repository.ActivityRow) []dto.ActivityItem {
	items := make([]dto.ActivityItem, len(rows))
	for i, r := range rows {
		items[i] = dto.ActivityItem{
			ID:        r.ID,
			UniqueID:  fmt.Sprintf("%s-%d", r.TypeStr, r.ID),
			Type:      r.TypeStr,
			Content:   r.Content,
			Link:      r.Link,
			Timestamp: r.Created,
			Actor: dto.Actor{
				ID: r.UserID,
			},
		}
	}
	return items
}

// enrichGalgameItems batch-fetches names for every galgame-scoped activity
// row from the galgame service and rewrites the content string per type:
//
//	GALGAME_CREATION          → "<game name>"
//	GALGAME_RESOURCE_CREATION → "在《<game name>》发布了下载资源"
//	GALGAME_RATING_CREATION   → "<game name> · <short summary>" (name only when no summary)
//
// GALGAME_COMMENT_CREATION is intentionally NOT rewritten: it keeps the raw
// comment text (matching the legacy API) since the type chip + link already
// convey what it is and where it points — a "在《game》" prefix is just noise.
//
// Galgame-scoped rows whose galgame has NO galgame brief for this viewer — NSFW in
// SFW mode, or deleted — are DROPPED from the returned slice, so an NSFW galgame
// never leaks into the public feed as "galgame#N / 未知用户". On galgame failure the
// items are returned untouched (graceful degradation).
//
// Resource-less GALGAME_CREATION rows are NOT handled here anymore — they're
// excluded in SQL (FetchFeed) so they never spend a LIMIT slot; see the repo.
//
// Returns the kept items (a subset of `items`); rows/items must be
// index-aligned on entry.
func (s *ActivityService) enrichGalgameItems(
	ctx context.Context,
	rows []repository.ActivityRow,
	items []dto.ActivityItem,
	isSFW bool,
) []dto.ActivityItem {
	idSet := map[int]struct{}{}
	for _, r := range rows {
		if r.GalgameID > 0 {
			idSet[r.GalgameID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return items
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	// Chunk: GetBatchPublic sends every id in one query param, and the over-fetch
	// window can surface more distinct galgames than a single batch should carry.
	briefMap := make(map[int]client.GalgameBrief, len(ids))
	for start := 0; start < len(ids); start += nextMoeBatchChunk {
		end := min(start+nextMoeBatchChunk, len(ids))
		m, appErr := s.galgameClient.GetBatchPublic(ctx, ids[start:end], isSFW)
		if appErr != nil {
			return items // graceful: galgame down, leave items untouched
		}
		maps.Copy(briefMap, m)
	}

	briefName := func(b client.GalgameBrief) string {
		// zh-cn > zh-tw > ja-jp > en-us — the FE getPreferredLanguageText zh-cn
		// default; en-US (VNDB romaji) last so a JP/CN game never shows it.
		for _, n := range []string{b.NameZhCn, b.NameZhTw, b.NameJaJp, b.NameEnUs} {
			if n != "" {
				return n
			}
		}
		return fmt.Sprintf("galgame#%d", b.ID)
	}

	preferredIntro := func(d client.GalgameDetailBrief) string {
		for _, s := range []string{d.IntroZhCN, d.IntroZhTW, d.IntroJaJP, d.IntroEnUS} {
			if s != "" {
				return s
			}
		}
		return ""
	}

	// Per-type extras: new-galgame counts (GALGAME_CREATION) + edit revision ids
	// (GALGAME_EDIT, for lazily loading the diff on the card).
	creationGIDs := make([]int, 0)
	editIDs := make([]int, 0)
	editGIDs := make([]int, 0)
	prGIDs := make([]int, 0)
	ratingIDs := make([]int, 0)
	for _, r := range rows {
		switch {
		case r.TypeStr == "GALGAME_CREATION" && r.GalgameID > 0:
			creationGIDs = append(creationGIDs, r.GalgameID)
		case r.TypeStr == "GALGAME_EDIT":
			editIDs = append(editIDs, r.ID)
			if r.GalgameID > 0 {
				editGIDs = append(editGIDs, r.GalgameID)
			}
		case r.TypeStr == "GALGAME_PR_CREATION" && r.GalgameID > 0:
			prGIDs = append(prGIDs, r.GalgameID)
		case r.TypeStr == "GALGAME_RATING_CREATION":
			ratingIDs = append(ratingIDs, r.ID)
		}
	}
	countsMap, _ := s.repo.FetchGalgameCounts(creationGIDs)
	revMap, _ := s.repo.FetchEditRevisions(editIDs)
	ratingMap, _ := s.repo.FetchRatingActivityData(ratingIDs)

	// Detail briefs (intro + officials + release date) for the cards that render
	// the full galgame info area — the new-galgame, edit AND PR cards, which share
	// that area. Best-effort: if galgame view=detail is unreachable, omitted.
	detailMap := map[int]client.GalgameDetailBrief{}
	if detailGIDs := append(append(append([]int{}, creationGIDs...), editGIDs...), prGIDs...); len(detailGIDs) > 0 {
		if m, appErr := s.galgameClient.GetBatchDetailPublic(ctx, detailGIDs, isSFW); appErr == nil {
			detailMap = m
		}
	}

	kept := make([]dto.ActivityItem, 0, len(items))
	for i, r := range rows {
		if r.GalgameID == 0 {
			kept = append(kept, items[i]) // not galgame-scoped: always keep
			continue
		}
		b, ok := briefMap[r.GalgameID]
		if !ok {
			// NSFW (SFW viewer) or deleted → drop the whole activity.
			continue
		}
		name := briefName(b)
		// Structured payload for the rich galgame cards (cover + name + meta),
		// straight from the brief in hand — Content is still rewritten below for
		// the generic/timeline card.
		ga := dto.GalgameActivityData{
			Name:        name,
			CoverHash:   b.EffectiveBannerHash,
			Language:    b.OriginalLanguage,
			AgeLimit:    b.AgeLimit,
			ReleaseDate: b.ReleaseDate,
			GalgameID:   r.GalgameID,
		}
		// Full info area (shared by the new-galgame, edit + PR cards): officials →
		// developer, detail release date, bounded ~3-line intro.
		if d, ok := detailMap[r.GalgameID]; ok &&
			(r.TypeStr == "GALGAME_CREATION" || r.TypeStr == "GALGAME_EDIT" ||
				r.TypeStr == "GALGAME_PR_CREATION") {
			ga.Developer = strings.Join(d.Officials, "、")
			ga.ReleaseDate = d.ReleaseDate // detail view carries it; brief omits
			if intro := []rune(preferredIntro(d)); len(intro) > 0 {
				if len(intro) > 300 {
					intro = intro[:300]
				}
				ga.Intro = string(intro)
			}
		}
		if r.TypeStr == "GALGAME_CREATION" {
			c := countsMap[r.GalgameID]
			ga.ResourceCount = c.ResourceCount
			ga.LikeCount = c.LikeCount
			ga.FavoriteCount = c.FavoriteCount
		}
		if r.TypeStr == "GALGAME_EDIT" {
			e := revMap[r.ID]
			ga.RevisionID = e.RevisionID         // legacy fallback (id→number)
			ga.RevisionNumber = e.RevisionNumber // diff endpoint's :rev (0 = unknown)
		}
		if r.TypeStr == "GALGAME_RATING_CREATION" {
			if rt, ok := ratingMap[r.ID]; ok {
				ga.Rating = &dto.RatingInfo{
					RatingID:     r.ID,
					Overall:      rt.Overall,
					PlayStatus:   rt.PlayStatus,
					Recommend:    rt.Recommend,
					ShortSummary: rt.ShortSummary,
					SpoilerLevel: rt.SpoilerLevel,
					LikeCount:    rt.LikeCount,
					AuthorID:     rt.AuthorID,
				}
			}
		}
		items[i].Data = ga
		switch r.TypeStr {
		case "GALGAME_CREATION":
			items[i].Content = name
			// The creator is the FROZEN wiki-era submitter on the LOCAL row
			// (migration 066), not the brief's user_id: the catalog face carries
			// no submitter, so that field is always 0 and this card silently lost
			// its actor (hydrateActors skips id 0, and the row then renders as an
			// actor-less system event). 0 here still means "unknown", which is the
			// pre-existing actor-less behaviour.
			if items[i].Actor.ID == 0 {
				items[i].Actor.ID = userclient.DerefID(countsMap[r.GalgameID].CreatorUserID)
			}
		case "GALGAME_RESOURCE_CREATION":
			items[i].Content = fmt.Sprintf("在《%s》发布了下载资源", name)
		case "GALGAME_EDIT":
			items[i].Content = fmt.Sprintf("编辑了《%s》", name)
		case "GALGAME_PR_CREATION":
			items[i].Content = fmt.Sprintf("对《%s》提出了更新请求", name)
		case "GALGAME_RATING_CREATION":
			if r.Content != "" {
				items[i].Content = fmt.Sprintf("%s · %s", name, r.Content)
			} else {
				items[i].Content = name
			}
		}
		kept = append(kept, items[i])
	}
	return kept
}

// hydrateActors batch-fetches identity (name/avatar) from OAuth for every
// non-zero actor id and writes it back into the items. Runs after
// enrichGalgameItems so GALGAME_CREATION rows whose actor was injected from
// the galgame brief are also hydrated.
//
// It also DROPS any item whose actor is banned and returns the surviving
// slice: the feed is a feed of user actions, so a banned user's actions must
// not surface (the galgame entity itself stays browsable via the catalogue).
// serveKeyset's fill-to-limit loop refetches to make up for the drops. Items
// with no actor (id 0 — system events) are always kept.
func (s *ActivityService) hydrateActors(ctx context.Context, items []dto.ActivityItem) []dto.ActivityItem {
	uids := userclient.CollectIDs(items, func(it dto.ActivityItem) int { return it.Actor.ID })
	if len(uids) == 0 {
		return items
	}
	userMap := s.userClient.Hydrate(ctx, uids)
	kept := make([]dto.ActivityItem, 0, len(items))
	for i := range items {
		u := userMap[items[i].Actor.ID]
		if items[i].Actor.ID > 0 && !userclient.IsRenderable(u) {
			continue
		}
		items[i].Actor.Name = u.Name
		items[i].Actor.Avatar = u.Avatar
		kept = append(kept, items[i])
	}
	return kept
}
