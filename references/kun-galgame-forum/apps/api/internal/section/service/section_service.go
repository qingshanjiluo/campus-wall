package service

import (
	"context"

	"kun-galgame-api/internal/section/dto"
	"kun-galgame-api/internal/section/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"
)

type SectionService struct {
	repo       *repository.SectionRepository
	userClient *userclient.Client
}

func NewSectionService(
	repo *repository.SectionRepository,
	userClient *userclient.Client,
) *SectionService {
	return &SectionService{repo: repo, userClient: userClient}
}

// GetSectionTopics returns topics filtered by section name. Identity is
// hydrated from OAuth via userclient since the repo no longer joins on the
// user table; banned authors are dropped from the listing.
func (s *SectionService) GetSectionTopics(ctx context.Context, req *dto.SectionTopicsRequest) (*dto.SectionTopicsResponse, *errors.AppError) {
	rows, total, err := s.repo.FindSectionTopics(req.Section, req.SortOrder, req.Page, req.Limit)
	if err != nil {
		return nil, errors.ErrInternal("获取板块话题失败")
	}

	uids := userclient.CollectIDs(rows, func(r repository.SectionTopicRow) int { return r.UserID })
	userMap := s.userClient.Hydrate(ctx, uids)

	items := make([]dto.SectionTopicItem, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		if !userclient.IsRenderable(u) {
			continue
		}
		items = append(items, dto.SectionTopicItem{
			ID: r.ID, Title: r.Title, Content: r.Content,
			View: r.View, LikeCount: r.LikeCount, ReplyCount: r.ReplyCount,
			HasBestAnswer: r.BestAnswerID != nil, IsNSFW: r.IsNSFW,
			User:    dto.UserBrief{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			Created: r.Created,
		})
	}

	return &dto.SectionTopicsResponse{Topics: items, Total: total}, nil
}

// GetCategoryStats returns section stats (topic count + view count + latest topic)
// filtered by category.
//
// The "latest topic" preview must not surface a banned author's topic (title +
// link), so we pull a few newest candidates per section and pick the newest
// whose author is renderable. The topic_count / view_count aggregates still
// include banned authors' topics — ban status is OAuth-owned, not a local
// column, so it can't be filtered in SQL; this is the same over-report every
// other list's `total` carries.
func (s *SectionService) GetCategoryStats(ctx context.Context, category string) ([]dto.SectionStat, *errors.AppError) {
	rows, err := s.repo.FindCategoryStats(category)
	if err != nil {
		return nil, errors.ErrInternal("获取板块统计失败")
	}

	// Fetch candidates for every section first, then hydrate all their authors
	// in one batch so the per-section pick is a cheap in-memory scan.
	const latestCandidates = 10
	candBySection := make(map[int][]repository.LatestTopicRow, len(rows))
	uidSet := map[int]struct{}{}
	for _, r := range rows {
		cands := s.repo.FindLatestTopicsInSection(r.SectionID, category, latestCandidates)
		candBySection[r.SectionID] = cands
		for _, c := range cands {
			uidSet[c.UserID] = struct{}{}
		}
	}
	uids := make([]int, 0, len(uidSet))
	for id := range uidSet {
		uids = append(uids, id)
	}
	userMap := s.userClient.Hydrate(ctx, uids)

	stats := make([]dto.SectionStat, len(rows))
	for i, r := range rows {
		stats[i] = dto.SectionStat{
			ID:         r.SectionID,
			Name:       r.SectionName,
			TopicCount: r.TopicCount,
			ViewCount:  r.ViewCount,
		}
		for _, c := range candBySection[r.SectionID] {
			if !userclient.IsRenderable(userMap[c.UserID]) {
				continue
			}
			stats[i].LatestTopic = &dto.LatestTopic{
				ID:      c.ID,
				Title:   c.Title,
				Created: c.Created,
			}
			break
		}
	}
	return stats, nil
}
