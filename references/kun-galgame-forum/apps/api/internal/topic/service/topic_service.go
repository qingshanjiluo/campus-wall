package service

import (
	"context"

	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/topic/repository"
	userRepo "kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
)

type TopicService struct {
	topicRepo    *repository.TopicRepository
	listRepo     *repository.TopicListRepository
	taxonomyRepo *repository.TopicTaxonomyRepository
	rdb          *redis.Client
	userClient   *userclient.Client
	stateRepo    *userRepo.StateRepository
}

func NewTopicService(
	topicRepo *repository.TopicRepository,
	listRepo *repository.TopicListRepository,
	taxonomyRepo *repository.TopicTaxonomyRepository,
	rdb *redis.Client,
	userClient *userclient.Client,
	stateRepo *userRepo.StateRepository,
) *TopicService {
	return &TopicService{
		topicRepo:    topicRepo,
		listRepo:     listRepo,
		taxonomyRepo: taxonomyRepo,
		rdb:          rdb,
		userClient:   userClient,
		stateRepo:    stateRepo,
	}
}

// GetMyInteractions returns the current user's favorited topic ids + reactions,
// for hydrating the feed card's 收藏 + reaction state client-side.
func (s *TopicService) GetMyInteractions(userID int) dto.MyTopicInteractions {
	favorited, reactions, err := s.topicRepo.UserTopicInteractions(userID)
	if err != nil {
		return dto.MyTopicInteractions{Favorited: []int{}, Reactions: map[int][]string{}}
	}
	return dto.MyTopicInteractions{Favorited: favorited, Reactions: reactions}
}

// topicUpvoteRecordLimit caps the push records shown below a topic (newest first).
const topicUpvoteRecordLimit = 50

// GetTopicUpvotes returns a topic's 推话题 records — who pushed it, their one-
// liner, and when — newest first, capped. Shown below the topic body.
func (s *TopicService) GetTopicUpvotes(ctx context.Context, topicID int) ([]dto.TopicUpvoteRecord, *errors.AppError) {
	rows, err := s.topicRepo.FetchTopicUpvotes(topicID, topicUpvoteRecordLimit)
	if err != nil {
		return nil, errors.ErrInternal("操作失败")
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	userMap := s.userClient.Hydrate(ctx, ids)
	out := make([]dto.TopicUpvoteRecord, 0, len(rows))
	for _, row := range rows {
		u := userMap[row.UserID]
		// Drop push records whose author is banned (each carries a one-liner).
		if !userclient.IsRenderable(u) {
			continue
		}
		out = append(out, dto.TopicUpvoteRecord{
			ID:          row.ID,
			User:        dto.KunUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			Description: row.Description,
			Created:     row.Created,
		})
	}
	return out, nil
}

// topicReactionHistoryLimit caps the reaction events shown in 查看历史 (newest
// first). Generous so it covers practically every topic; Hydrate auto-shards the
// reactor ids into ≤100-id batches, so a large cap stays safe.
const topicReactionHistoryLimit = 300

// GetTopicReactionHistory returns a topic's reaction events — who reacted, with
// which reaction, and when — newest first, capped. Powers the 查看历史 modal.
func (s *TopicService) GetTopicReactionHistory(ctx context.Context, topicID int) ([]dto.ReactionHistoryItem, *errors.AppError) {
	rows, err := s.topicRepo.GetTopicReactionHistory(topicID, topicReactionHistoryLimit)
	if err != nil {
		return nil, errors.ErrInternal("操作失败")
	}
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	userMap := s.userClient.Hydrate(ctx, ids)
	out := make([]dto.ReactionHistoryItem, 0, len(rows))
	for _, row := range rows {
		u := userMap[row.UserID]
		out = append(out, dto.ReactionHistoryItem{
			User:     dto.KunUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar},
			Reaction: row.Reaction,
			Created:  row.Created,
		})
	}
	return out, nil
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

func (s *TopicService) GetList(
	ctx context.Context,
	req *dto.ListTopicsRequest,
	isNSFW bool,
) ([]dto.TopicCard, int64, *errors.AppError) {
	rows, total, err := s.listRepo.FindList(
		req.Page, req.Limit,
		req.SortField, req.SortOrder, req.Category,
		isNSFW,
	)
	if err != nil {
		return nil, 0, errors.ErrInternal("获取话题列表失败")
	}

	return s.mapListRows(ctx, rows, total)
}

func (s *TopicService) GetResourceList(
	ctx context.Context,
	req *dto.ListTopicsRequest,
	isNSFW bool,
) ([]dto.TopicCard, int64, *errors.AppError) {
	rows, total, err := s.listRepo.FindResourceList(
		req.Page, req.Limit,
		req.SortField, req.SortOrder, req.Category,
		isNSFW,
	)
	if err != nil {
		return nil, 0, errors.ErrInternal("获取资源话题列表失败")
	}

	return s.mapListRows(ctx, rows, total)
}

// mapListRows enriches topic card rows with sections and maps to DTOs.
// Identity (UserName/UserAvatar) is hydrated from OAuth via userclient since
// kungal no longer keeps a local users table; rows authored by a banned user
// are dropped from the listing (see the IsRenderable skip below).
func (s *TopicService) mapListRows(ctx context.Context, rows []repository.TopicCardRow, total int64) ([]dto.TopicCard, int64, *errors.AppError) {
	topicIDs := make([]int, len(rows))
	for i, r := range rows {
		topicIDs[i] = r.ID
	}

	sectionMap, _ := s.taxonomyRepo.FindSectionNamesByTopicIDs(topicIDs)
	// Batch-check which of these topics actually have a poll attached.
	// Without this the cards' `isPollTopic` was always `false`, so the
	// "投票" badge never showed on /topic or /resource list pages.
	pollSet := s.topicRepo.FindTopicIDsWithPoll(topicIDs)

	uids := userclient.CollectIDs(rows, func(r repository.TopicCardRow) int { return r.UserID })
	userMap := s.userClient.Hydrate(ctx, uids)
	for i := range rows {
		u := userMap[rows[i].UserID]
		rows[i].UserName = u.Name
		rows[i].UserAvatar = u.Avatar
	}

	cards := make([]dto.TopicCard, 0, len(rows))
	for i, r := range rows {
		// Drop banned authors' content from the listing.
		if u, ok := userMap[r.UserID]; ok && !userclient.IsRenderable(u) {
			continue
		}
		cards = append(cards, toTopicCard(r, sectionMap[r.ID], pollSet[r.ID]))
		_ = i
	}
	return cards, total, nil
}

// ──────────────────────────────────────────
// Detail
// ──────────────────────────────────────────

func (s *TopicService) GetDetail(
	ctx context.Context,
	topicID int,
	userInfo *middleware.UserInfo,
) (*dto.TopicDetail, *errors.AppError) {
	topic, err := s.topicRepo.FindByID(topicID)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该话题")
	}

	g, _ := errgroup.WithContext(ctx)

	var author *repository.TopicAuthorUser
	var authorBanned bool
	var sections []string
	var hasPoll bool
	var isLiked, isDisliked, isFavorited, isUpvoted bool

	g.Go(func() error {
		// Identity from OAuth, moemoepoint from kungal_user_state.
		u, _, e := s.userClient.User(ctx, topic.UserID)
		if e != nil {
			return e
		}
		// A banned author's topic is hidden even by direct link (404 below).
		if !userclient.IsRenderable(u) {
			authorBanned = true
			return nil
		}
		moe := 0
		if state, _ := s.stateRepo.FindByID(topic.UserID); state != nil {
			moe = state.Moemoepoint
		}
		author = &repository.TopicAuthorUser{
			ID: u.ID, Name: u.Name, Avatar: u.Avatar, Moemoepoint: moe,
		}
		return nil
	})
	g.Go(func() error {
		var e error
		sections, e = s.taxonomyRepo.FindSectionNamesByTopicID(topicID)
		return e
	})
	g.Go(func() error {
		var e error
		hasPoll, e = s.topicRepo.HasPoll(topicID)
		return e
	})

	if userInfo != nil {
		userID := userInfo.ID
		g.Go(func() error {
			isLiked, _ = s.topicRepo.HasUserLiked(userID, topicID)
			return nil
		})
		g.Go(func() error {
			isDisliked, _ = s.topicRepo.HasUserDisliked(userID, topicID)
			return nil
		})
		g.Go(func() error {
			isFavorited, _ = s.topicRepo.HasUserFavorited(userID, topicID)
			return nil
		})
		g.Go(func() error {
			isUpvoted, _ = s.topicRepo.HasUserUpvoted(userID, topicID)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, errors.ErrInternal("获取话题详情失败")
	}
	// Banned author → hide the whole topic (before counting a view). `author`
	// is left nil in this case, so this must precede the DTO assembly below.
	if authorBanned {
		return nil, errors.ErrNotFound("未找到该话题")
	}

	// Increment view asynchronously
	go s.topicRepo.IncrementView(topicID)

	if sections == nil {
		sections = []string{}
	}
	covers := []string(topic.CoverImages)
	if covers == nil {
		covers = []string{}
	}

	// Resolve @mention display names in the topic body to the authors' CURRENT
	// names (one batch), so a renamed user shows their new name; unresolved ids
	// keep the write-time snapshot.
	topicMentionNames := map[int]string{}
	if ids := markdown.ExtractMentionIDs(topic.Content); len(ids) > 0 {
		for id, u := range s.userClient.Hydrate(ctx, ids) {
			topicMentionNames[id] = u.Name
		}
	}

	detail := &dto.TopicDetail{
		ID:          topic.ID,
		Title:       topic.Title,
		Content:     topic.Content,
		ContentHtml: markdown.ResolveMentionNames(markdown.Render(topic.Content), topicMentionNames),
		View:        topic.View,
		Status:      topic.Status,
		IsNSFW:      topic.IsNSFW,
		Category:    topic.Category,
		Sections:    sections,
		CoverImages: covers,
		// Reserve each cover's aspect ratio (no CLS) + blur-up on the FE.
		CoverImageMeta: markdown.ResolveContentImageMeta(covers),
		User: dto.KunUserWithMoemoepoint{
			ID:          author.ID,
			Name:        author.Name,
			Avatar:      author.Avatar,
			Moemoepoint: author.Moemoepoint,
		},
		LikeCount:        topic.LikeCount,
		IsLiked:          isLiked,
		DislikeCount:     topic.DislikeCount,
		IsDisliked:       isDisliked,
		FavoriteCount:    topic.FavoriteCount,
		IsFavorited:      isFavorited,
		UpvoteCount:      topic.UpvoteCount,
		IsUpvoted:        isUpvoted,
		ReplyCount:       topic.ReplyCount,
		IsPollTopic:      hasPoll,
		StatusUpdateTime: topic.StatusUpdateTime,
		UpvoteTime:       topic.UpvoteTime,
		Edited:           topic.Edited,
		Created:          topic.CreatedAt,
	}

	// Reactions: per-key counts + the viewer's own reactions + (for small counts)
	// reactor avatars.
	viewerID := 0
	if userInfo != nil {
		viewerID = userInfo.ID
	}
	rrows, _ := s.topicRepo.GetTopicReactions(topicID)
	mineKeys, _ := s.topicRepo.GetUserTopicReactions(topicID, viewerID)
	detail.Reactions = buildReactionSummaries(
		rrows, mineKeys, s.userClient.Hydrate(ctx, reactionReactorIDs(rrows)))

	// Hydrate best-answer summary (JSON-LD acceptedAnswer on FE side).
	// Identity comes from OAuth via userClient — same path as the topic
	// author. Errors are tolerated: a broken best-answer reply must not
	// break the whole detail page.
	if topic.BestAnswerID != nil {
		reply, replyErr := s.topicRepo.FindReplyByID(*topic.BestAnswerID)
		if replyErr == nil && reply != nil {
			ru, _, _ := s.userClient.User(ctx, reply.UserID)
			// Omit the best-answer highlight when its author is banned — the
			// same reply is already dropped from the paginated reply list.
			if userclient.IsRenderable(ru) {
				baMentionNames := map[int]string{}
				if ids := markdown.ExtractMentionIDs(reply.Content); len(ids) > 0 {
					for id, u := range s.userClient.Hydrate(ctx, ids) {
						baMentionNames[id] = u.Name
					}
				}
				detail.BestAnswer = &dto.TopicBestAnswer{
					ID:              reply.ID,
					Floor:           reply.Floor,
					User:            dto.KunUser{ID: ru.ID, Name: ru.Name, Avatar: ru.Avatar},
					ContentMarkdown: reply.Content,
					ContentHtml:     markdown.ResolveMentionNames(markdown.Render(reply.Content), baMentionNames),
					Created:         reply.CreatedAt,
				}
			}
		}
	}

	return detail, nil
}
