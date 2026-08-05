package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/infrastructure/markdown"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/topic/dto"
	topicModel "kun-galgame-api/internal/topic/model"
	"kun-galgame-api/internal/topic/repository"
	"kun-galgame-api/internal/trust/gate"
	userRepo "kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ReplyService struct {
	replyRepo   *repository.ReplyRepository
	commentRepo *repository.CommentRepository
	topicRepo   *repository.TopicRepository
	stateRepo   *userRepo.StateRepository
	userClient  *userclient.Client
	rdb         *redis.Client
	check       *gate.CheckService
	scan        *gate.ScanService
	helpers     InteractionHelpers
}

func NewReplyService(
	replyRepo *repository.ReplyRepository,
	commentRepo *repository.CommentRepository,
	topicRepo *repository.TopicRepository,
	stateRepo *userRepo.StateRepository,
	userClient *userclient.Client,
	rdb *redis.Client,
	check *gate.CheckService,
	scan *gate.ScanService,
) *ReplyService {
	return &ReplyService{
		replyRepo:   replyRepo,
		commentRepo: commentRepo,
		topicRepo:   topicRepo,
		stateRepo:   stateRepo,
		userClient:  userClient,
		rdb:         rdb,
		check:       check,
		scan:        scan,
	}
}

// ──────────────────────────────────────────
// Locate (deep-link)
// ──────────────────────────────────────────

// LocateReply resolves a deep-link target — a reply floor OR a comment id — to the
// reply-stream page it lives on, so the frontend can load that page directly and
// scroll to it. commentID wins: its parent reply's floor is resolved first. Returns
// ErrNotFound when a comment target no longer exists; a deleted reply floor just
// yields the page where it would sit (the frontend gracefully no-ops the scroll).
func (s *ReplyService) LocateReply(topicID, floor, commentID, limit int) (*dto.ReplyLocateResponse, *errors.AppError) {
	replyID := 0
	if commentID > 0 {
		f, rid, ok, err := s.replyRepo.FindReplyFloorByCommentID(topicID, commentID)
		if err != nil {
			return nil, errors.ErrInternal("定位评论失败")
		}
		if !ok {
			return nil, errors.ErrNotFound("评论不存在或已删除")
		}
		floor, replyID = f, rid
	}
	if floor <= 0 {
		return nil, errors.ErrBadRequest("缺少 reply 或 comment 参数")
	}
	page, err := s.replyRepo.LocateReplyPageByFloor(topicID, floor, limit)
	if err != nil {
		return nil, errors.ErrInternal("定位回复失败")
	}
	return &dto.ReplyLocateResponse{
		Page:      page,
		Floor:     floor,
		ReplyID:   replyID,
		CommentID: commentID,
	}, nil
}

// ──────────────────────────────────────────
// List replies
// ──────────────────────────────────────────

func (s *ReplyService) GetReplies(
	ctx context.Context,
	req *dto.ListRepliesRequest,
	userInfo *middleware.UserInfo,
) ([]dto.TopicReplyResponse, *errors.AppError) {
	topic, err := s.topicRepo.FindByID(req.TopicID)
	if err != nil {
		return []dto.TopicReplyResponse{}, nil
	}

	// Collect special reply IDs (pinned + best answer)
	var specialIDs []int
	if topic.PinnedReplyID != nil {
		specialIDs = append(specialIDs, *topic.PinnedReplyID)
	}
	if topic.BestAnswerID != nil && (topic.PinnedReplyID == nil || *topic.BestAnswerID != *topic.PinnedReplyID) {
		specialIDs = append(specialIDs, *topic.BestAnswerID)
	}

	var result []dto.TopicReplyResponse

	// On page 1, prepend special replies
	if req.Page == 1 && len(specialIDs) > 0 {
		if specialRows, err := s.replyRepo.FindRepliesByIDs(specialIDs); err == nil {
			result = append(result, s.buildReplyResponses(ctx, specialRows, topic, userInfo)...)
		}
	}

	regularRows, err := s.replyRepo.FindRepliesPaginated(
		req.TopicID, specialIDs,
		req.Page, req.Limit, req.SortOrder,
	)
	if err != nil {
		return nil, errors.ErrInternal("获取回复列表失败")
	}

	result = append(result, s.buildReplyResponses(ctx, regularRows, topic, userInfo)...)

	if result == nil {
		result = []dto.TopicReplyResponse{}
	}
	return result, nil
}

// ──────────────────────────────────────────
// Reply detail
// ──────────────────────────────────────────

func (s *ReplyService) GetReplyDetail(
	ctx context.Context,
	replyID int,
	userInfo *middleware.UserInfo,
) (*dto.TopicReplyResponse, *errors.AppError) {
	rows, err := s.replyRepo.FindRepliesByIDs([]int{replyID})
	if err != nil || len(rows) == 0 {
		return nil, errors.ErrNotFound("未找到该回复")
	}

	topic, _ := s.topicRepo.FindByID(rows[0].TopicID)
	responses := s.buildReplyResponses(ctx, rows, topic, userInfo)
	if len(responses) == 0 {
		return nil, errors.ErrNotFound("未找到该回复")
	}
	return &responses[0], nil
}

// ──────────────────────────────────────────
// Create reply — floor calculation inside tx
// ──────────────────────────────────────────

func (s *ReplyService) CreateReply(
	ctx context.Context,
	userID int,
	req *dto.CreateReplyRequest,
) (*dto.TopicReplyResponse, *errors.AppError) {
	topic, err := s.topicRepo.FindByID(req.TopicID)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该话题")
	}

	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.ErrBadRequest("回复内容不能为空")
	}

	// Synchronous word-list gate BEFORE the tx (trust wave 1). A reply's
	// moderation text is its RAW body (no title). deny blocks (nothing persisted);
	// hold publishes normally + logs; fail-open on any error/timeout.
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, req.Content, &authorID)
	if decision == gate.DecisionDeny {
		return nil, errContentBlocked()
	}

	var newReply *topicModel.TopicReply

	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		maxFloor, err := s.replyRepo.GetMaxFloor(tx, req.TopicID)
		if err != nil {
			return err
		}

		newReply = &topicModel.TopicReply{
			UserID:  userID,
			TopicID: req.TopicID,
			Floor:   maxFloor + 1,
			Content: req.Content,
		}
		if err := s.replyRepo.CreateReply(tx, newReply); err != nil {
			return err
		}

		if err := s.topicRepo.TouchStatusUpdateTime(tx, req.TopicID, time.Now()); err != nil {
			return err
		}

		if err := recomputeTopicCounts(tx, req.TopicID); err != nil {
			return err
		}

		preview := truncate(strings.TrimSpace(req.Content), constants.TextPreviewLength)

		// Notify the topic owner their topic got a reply (independent of mentions).
		if topic.UserID != userID {
			s.helpers.AdjustMoemoepoint(tx, topic.UserID, constants.RewardReply,
				moemoepoint.ReasonContentApproved, moemoepoint.Ref("topic", req.TopicID))
			s.helpers.CreateReplyMessage(tx, userID, topic.UserID, "replied", preview, req.TopicID, newReply.Floor, 0)
		}

		// @mentions in the reply body → "mentioned" notifications (deduped, self
		// skipped). Replying-to-a-floor now flows through here: the 「引用」 button
		// inserts an @mention of the quoted author, so they're notified as
		// "mentioned" in place of the retired per-target "replied".
		s.helpers.NotifyMentions(tx, userID, req.TopicID, newReply.Floor, 0, req.Content)

		return nil
	})

	if txErr != nil {
		return nil, errors.ErrInternal("创建回复失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindReply, "subject_id", newReply.ID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindReply, strconv.Itoa(newReply.ID), req.Content, int64(userID))

	rows, _ := s.replyRepo.FindRepliesByIDs([]int{newReply.ID})
	if len(rows) == 0 {
		return nil, errors.ErrInternal("创建回复失败")
	}
	responses := s.buildReplyResponses(ctx, rows, topic, nil)
	return &responses[0], nil
}

// ──────────────────────────────────────────
// Update reply
// ──────────────────────────────────────────

func (s *ReplyService) UpdateReply(
	ctx context.Context,
	userID int,
	req *dto.UpdateReplyRequest,
) *errors.AppError {
	reply, err := s.replyRepo.FindByID(req.ReplyID)
	if err != nil {
		return errors.ErrNotFound("未找到该回复")
	}
	if reply.UserID != userID {
		return errors.ErrForbidden("您没有权限编辑此回复")
	}

	if strings.TrimSpace(req.Content) == "" {
		return errors.ErrBadRequest("回复内容不能为空")
	}

	// Synchronous word-list gate BEFORE the tx (deny blocks, hold publishes+logs,
	// fail-open). author_id is the content author (reply.UserID).
	authorID := int64(reply.UserID)
	decision, matched := s.check.Decision(ctx, req.Content, &authorID)
	if decision == gate.DecisionDeny {
		return errContentBlocked()
	}

	now := time.Now()
	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.replyRepo.UpdateReplyContent(tx, req.ReplyID, map[string]any{
			"content": req.Content,
			"edited":  &now,
		}); err != nil {
			return err
		}

		// @mentions in the edited reply → notify newly mentioned users (deduped,
		// so anyone already mentioned in this topic isn't re-notified on edit).
		s.helpers.NotifyMentions(tx, userID, reply.TopicID, reply.Floor, 0, req.Content)

		return nil
	})

	if txErr != nil {
		return errors.ErrInternal("更新回复失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindReply, "subject_id", req.ReplyID, "author_id", reply.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindReply, strconv.Itoa(req.ReplyID), req.Content, int64(reply.UserID))

	return nil
}

// ──────────────────────────────────────────
// Delete reply — cascade + moemoepoint penalty
// ──────────────────────────────────────────

func (s *ReplyService) DeleteReply(
	ctx context.Context,
	userID int,
	canModerate bool,
	replyID int,
) *errors.AppError {
	reply, err := s.replyRepo.FindByID(replyID)
	if err != nil {
		return errors.ErrNotFound("未找到该回复")
	}
	if reply.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限删除此回复")
	}

	commentCount, likeCount, _ := s.replyRepo.CountReplyRelated(replyID)

	penalty := 3
	if reply.UserID == userID && !canModerate {
		penalty = 3 * int(commentCount+likeCount+1)
	}

	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		state, err := s.stateRepo.LockForUpdate(tx, reply.UserID)
		if err != nil {
			return err
		}
		if state.Moemoepoint < penalty {
			return gorm.ErrCheckConstraintViolated
		}

		if err := s.replyRepo.DeleteRepliesByIDs(tx, []int{replyID}); err != nil {
			return err
		}

		if err := recomputeTopicCounts(tx, reply.TopicID); err != nil {
			return err
		}

		s.helpers.AdjustMoemoepoint(tx, reply.UserID, -penalty,
			moemoepoint.ReasonContentRemoved, moemoepoint.Ref("topic_reply", replyID))
		return nil
	})

	if txErr == gorm.ErrCheckConstraintViolated {
		return errors.ErrBadRequest("萌萌点不足, 无法删除此回复")
	}
	if txErr != nil {
		return errors.ErrInternal("删除回复失败")
	}
	return nil
}

// ModerationRemove hard-deletes a reply for a T&S `remove` enforcement — no
// permission check (the disposition is already authorized) and NO moemoepoint
// penalty (unlike DeleteReply, which can even abort on insufficient author
// points). Idempotent: a missing reply is a no-op.
func (s *ReplyService) ModerationRemove(replyID int) error {
	reply, err := s.replyRepo.FindByID(replyID)
	if err != nil {
		return nil
	}
	return s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.replyRepo.DeleteRepliesByIDs(tx, []int{replyID}); err != nil {
			return err
		}
		return recomputeTopicCounts(tx, reply.TopicID)
	})
}

// ──────────────────────────────────────────
// Reply interactions
// ──────────────────────────────────────────

// ToggleReplyLike / ToggleReplyDislike are kept as thin aliases (the legacy
// endpoints still call them) — both route through the unified reaction path.
func (s *ReplyService) ToggleReplyLike(ctx context.Context, userID, replyID int) *errors.AppError {
	return s.ToggleReplyReaction(ctx, userID, replyID, "like")
}

func (s *ReplyService) ToggleReplyDislike(ctx context.Context, userID, replyID int) *errors.AppError {
	return s.ToggleReplyReaction(ctx, userID, replyID, "dislike")
}

// ToggleReplyReaction is the reply-level counterpart of TopicWriteService
// .ToggleReaction: like → ±1 moemoepoint to the reply owner + a "liked"
// notification; like⇄dislike mutually exclusive; emoji reactions plain. Only
// 'like' is blocked on one's own reply. (reactionKeys lives in topic_write_service.go.)
func (s *ReplyService) ToggleReplyReaction(ctx context.Context, userID, replyID int, reaction string) *errors.AppError {
	if !reactionKeys[reaction] {
		return errors.ErrBadRequest("无效的 reaction")
	}
	err := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		reply, err := s.replyRepo.FindByIDTx(tx, replyID)
		if err != nil {
			return err
		}
		if reaction == "like" && reply.UserID == userID {
			return gorm.ErrInvalidData
		}

		has, err := s.replyRepo.HasReplyReaction(tx, replyID, userID, reaction)
		if err != nil {
			return err
		}
		if has {
			if err := s.replyRepo.RemoveReplyReaction(tx, replyID, userID, reaction); err != nil {
				return err
			}
			switch reaction {
			case "like":
				if err := s.replyRepo.AdjustReplyLikeCount(tx, replyID, -1); err != nil {
					return err
				}
				s.helpers.AdjustMoemoepoint(tx, reply.UserID, -1,
					moemoepoint.ReasonLiked, moemoepoint.Ref("topic_reply", replyID))
			case "dislike":
				if err := s.replyRepo.AdjustReplyDislikeCount(tx, replyID, -1); err != nil {
					return err
				}
			}
			return nil
		}

		switch reaction {
		case "like":
			if err := s.clearReplyReaction(tx, replyID, userID, reply.UserID, "dislike"); err != nil {
				return err
			}
			if err := s.replyRepo.AddReplyReaction(tx, replyID, userID, "like"); err != nil {
				return err
			}
			if err := s.replyRepo.AdjustReplyLikeCount(tx, replyID, 1); err != nil {
				return err
			}
			s.helpers.AdjustMoemoepoint(tx, reply.UserID, 1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("topic_reply", replyID))
			link := msgService.BuildTopicLink(reply.TopicID, reply.Floor, 0)
			createDedupMessage(tx, userID, reply.UserID, "liked",
				truncate(reply.Content, constants.TextPreviewLength), link)
		case "dislike":
			if err := s.clearReplyReaction(tx, replyID, userID, reply.UserID, "like"); err != nil {
				return err
			}
			if err := s.replyRepo.AddReplyReaction(tx, replyID, userID, "dislike"); err != nil {
				return err
			}
			if err := s.replyRepo.AdjustReplyDislikeCount(tx, replyID, 1); err != nil {
				return err
			}
		default:
			if err := s.replyRepo.AddReplyReaction(tx, replyID, userID, reaction); err != nil {
				return err
			}
		}
		return nil
	})

	switch err {
	case nil:
		return nil
	case gorm.ErrInvalidData:
		return errors.ErrBadRequest("您不能给自己的回复点赞")
	default:
		return errors.ErrInternal("操作失败")
	}
}

// clearReplyReaction removes the user's `reaction` on a reply if present (like⇄
// dislike exclusion), reversing the like count + moemoepoint for a 'like'.
func (s *ReplyService) clearReplyReaction(tx *gorm.DB, replyID, userID, ownerID int, reaction string) error {
	has, err := s.replyRepo.HasReplyReaction(tx, replyID, userID, reaction)
	if err != nil || !has {
		return err
	}
	if err := s.replyRepo.RemoveReplyReaction(tx, replyID, userID, reaction); err != nil {
		return err
	}
	switch reaction {
	case "like":
		if err := s.replyRepo.AdjustReplyLikeCount(tx, replyID, -1); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, ownerID, -1,
			moemoepoint.ReasonLiked, moemoepoint.Ref("topic_reply", replyID))
	case "dislike":
		if err := s.replyRepo.AdjustReplyDislikeCount(tx, replyID, -1); err != nil {
			return err
		}
	}
	return nil
}

func (s *ReplyService) PinReply(ctx context.Context, userID int, canModerate bool, topicID, replyID int) *errors.AppError {
	topic, err := s.topicRepo.FindByID(topicID)
	if err != nil {
		return errors.ErrNotFound("未找到该话题")
	}
	if topic.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限置顶回复")
	}

	reply, err := s.replyRepo.FindByID(replyID)
	if err != nil {
		return errors.ErrNotFound("未找到该回复")
	}

	isPinning := topic.PinnedReplyID == nil || *topic.PinnedReplyID != replyID
	var newPinned *int
	if isPinning {
		newPinned = &replyID
	}

	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&topicModel.Topic{}).Where("id = ?", topicID).
			Updates(map[string]any{"pinned_reply_id": newPinned}).Error; err != nil {
			return err
		}
		if isPinning && userID != reply.UserID {
			s.helpers.CreateTopicMessageWithContent(
				tx, userID, reply.UserID, "pin-reply",
				replyPlainPreview(*reply),
				topicID, reply.Floor, 0,
			)
		}
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

// replyPlainPreview is the reply's markdown-stripped content, used for pin-reply
// / solution notification previews. (The Phase-4 migration folded legacy
// multi-target content into Content, so there are no separate targets to append.)
func replyPlainPreview(reply topicModel.TopicReply) string {
	return markdown.ToPlainText(reply.Content, 500)
}
