package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/home/dto"
	"kun-galgame-api/internal/home/repository"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
)

const (
	// Public count — the number of galgame cards the homepage actually renders.
	homeGalgameLimit = 12
	// Over-fetch factor: galgame may be missing a subset of local IDs (migration
	// lag) and the SFW filter drops some too, so we need slack above
	// homeGalgameLimit to reliably return `homeGalgameLimit` usable cards.
	homeGalgameFetchLimit = 24
	homeTopicLimit        = 10
)

// homeCacheTTL bounds homepage staleness. The page has no per-user state (keyed
// only by isSFW), so one cached build is shared across viewers, sparing the
// per-render galgame-list query + galgame name-enrichment round-trip.
const homeCacheTTL = 60 * time.Second

type HomeService struct {
	repo          *repository.HomeRepository
	galgameClient *client.GalgameClient
	userClient    *userclient.Client
	rdb           *redis.Client
}

func NewHomeService(
	repo *repository.HomeRepository,
	gc *client.GalgameClient,
	userClient *userclient.Client,
	rdb *redis.Client,
) *HomeService {
	return &HomeService{repo: repo, galgameClient: gc, userClient: userClient, rdb: rdb}
}

// GetHome returns homepage data: galgames + topics.
//
// Galgame data is best-effort: if the galgame service is unreachable we log and
// return an empty galgame list rather than failing the whole endpoint, so the
// topic feed still renders. Topic lookup failures are still propagated — they
// come from the local DB and indicate something more serious.
func (s *HomeService) GetHome(ctx context.Context, isSFW bool) (*dto.HomeResponse, error) {
	cacheKey := fmt.Sprintf("home:v1:%t", isSFW)
	if cached := s.getCachedHome(ctx, cacheKey); cached != nil {
		return cached, nil
	}

	galgames, gErr := s.getHomeGalgames(ctx, isSFW)
	if gErr != nil {
		slog.Warn("首页 galgame 获取失败, 降级为空列表", "error", gErr)
		galgames = []dto.HomeGalgame{}
	}
	topics, err := s.getHomeTopics(ctx, isSFW)
	if err != nil {
		return nil, err
	}
	resp := &dto.HomeResponse{Galgames: galgames, Topics: topics}
	// Only cache a fully-successful build — never pin a galgame-degraded (empty
	// galgame) homepage for the whole TTL.
	if gErr == nil {
		s.cacheHome(ctx, cacheKey, resp)
	}
	return resp, nil
}

// getCachedHome returns the cached homepage, or nil on a miss. Fail-open: any
// redis/JSON error is a miss so the request still serves fresh data.
func (s *HomeService) getCachedHome(ctx context.Context, key string) *dto.HomeResponse {
	if s.rdb == nil {
		return nil
	}
	raw, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}
	var resp dto.HomeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	return &resp
}

// cacheHome best-effort stores the homepage with homeCacheTTL. A redis/marshal
// failure is ignored — caching must never break the response.
func (s *HomeService) cacheHome(ctx context.Context, key string, resp *dto.HomeResponse) {
	if s.rdb == nil || resp == nil {
		return
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, key, raw, homeCacheTTL).Err()
}

func (s *HomeService) getHomeGalgames(ctx context.Context, isSFW bool) ([]dto.HomeGalgame, error) {
	// Step 1: Over-fetch local rows. Galgame may not know some IDs (migration
	// lag) and the SFW filter below also drops rows, so we pull 2x the
	// displayed count to reliably land at `homeGalgameLimit` after filtering.
	localRows, err := s.repo.FindRecentGalgames(homeGalgameFetchLimit)
	if err != nil {
		return nil, err
	}
	if len(localRows) == 0 {
		return []dto.HomeGalgame{}, nil
	}

	// Step 2: Batch fetch metadata from galgame
	galgameIDs := make([]int, len(localRows))
	for i, r := range localRows {
		galgameIDs[i] = r.ID
	}
	// Galgame-side SFW filter — see docs/galgame_wiki/00-handbook §16. The
	// batch endpoint defaults to NO filter, so we must pass isSFW
	// explicitly. Any post-filter at this layer would be wrong per spec.
	briefMap, appErr := s.galgameClient.GetBatchPublic(ctx, galgameIDs, isSFW)
	if appErr != nil {
		return nil, appErr
	}

	// Step 3: Batch fetch users from OAuth. The author chip is keyed by the
	// FROZEN wiki-era creator on the LOCAL row (migration 066): the catalog face
	// carries no submitter, so a brief's own user_id is always 0 — hydrating off
	// it asked OAuth about user 0 and rendered every home card as 已注销用户.
	userMap := s.userClient.Hydrate(ctx, userclient.CollectIDs(localRows,
		func(lr repository.GalgameLocalRow) int { return userclient.DerefID(lr.CreatorUserID) }))

	// Step 4: Batch fetch platforms/languages from local galgame_resource
	resources := s.repo.FindResourcePlatformLanguage(galgameIDs)
	platformMap := map[int]map[string]bool{}
	languageMap := map[int]map[string]bool{}
	for _, r := range resources {
		if platformMap[r.GalgameID] == nil {
			platformMap[r.GalgameID] = map[string]bool{}
		}
		if languageMap[r.GalgameID] == nil {
			languageMap[r.GalgameID] = map[string]bool{}
		}
		platformMap[r.GalgameID][r.Platform] = true
		languageMap[r.GalgameID][r.Language] = true
	}

	// Step 5: Assemble results in original order, stopping at homeGalgameLimit.
	result := make([]dto.HomeGalgame, 0, homeGalgameLimit)
	for _, lr := range localRows {
		if len(result) >= homeGalgameLimit {
			break
		}
		b, ok := briefMap[lr.ID]
		if !ok {
			continue // galgame doesn't have this galgame
		}
		u := userMap[userclient.DerefID(lr.CreatorUserID)]
		result = append(result, dto.HomeGalgame{
			ID: lr.ID,
			Name: dto.LocaleName{
				EnUS: b.NameEnUs, JaJP: b.NameJaJp,
				ZhCN: b.NameZhCn, ZhTW: b.NameZhTw,
			},
			Banner:             b.Banner,
			User:               dto.UserBrief{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			ContentLimit:       b.ContentLimit,
			View:               lr.View,
			LikeCount:          lr.LikeCount,
			ResourceUpdateTime: lr.ResourceUpdateTime.Format(time.RFC3339),
			Platform:           mapKeys(platformMap[lr.ID]),
			Language:           mapKeys(languageMap[lr.ID]),
			// U2: pass through the derived banner so the FE card can
			// pick `_mini`. Without these, new (covers-only) galgames
			// fall through to the empty legacy `banner` field.
			EffectiveBannerHash: b.EffectiveBannerHash,
			EffectiveBannerURL:  b.EffectiveBannerURL,
		})
	}

	return result, nil
}

func (s *HomeService) getHomeTopics(ctx context.Context, isSFW bool) ([]dto.HomeTopic, error) {
	rows, err := s.repo.FindHomeTopics(homeTopicLimit, isSFW)
	if err != nil {
		return nil, err
	}

	topicIDs := make([]int, len(rows))
	for i, r := range rows {
		topicIDs[i] = r.ID
	}

	sections := s.repo.FindTopicSections(topicIDs)
	sectionMap := map[int][]string{}
	for _, sct := range sections {
		sectionMap[sct.TopicID] = append(sectionMap[sct.TopicID], sct.SectionName)
	}

	pollSet := s.repo.FindTopicIDsWithPoll(topicIDs)

	uids := userclient.CollectIDs(rows, func(r repository.TopicRow) int { return r.UserID })
	userMap := s.userClient.Hydrate(ctx, uids)

	result := make([]dto.HomeTopic, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		topicSections := sectionMap[r.ID]
		if topicSections == nil {
			topicSections = []string{}
		}

		result = append(result, dto.HomeTopic{
			ID:               r.ID,
			Title:            r.Title,
			View:             r.View,
			LikeCount:        r.LikeCount,
			ReplyCount:       r.ReplyCount,
			CommentCount:     r.CommentCount,
			HasBestAnswer:    r.BestAnswerID != nil,
			IsPollTopic:      pollSet[r.ID],
			IsNSFWTopic:      r.IsNSFW,
			Section:          topicSections,
			User:             dto.UserBrief{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			Status:           r.Status,
			UpvoteTime:       r.UpvoteTime,
			StatusUpdateTime: r.StatusUpdateTime,
		})
	}

	return result, nil
}

func mapKeys(m map[string]bool) []string {
	if m == nil {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
