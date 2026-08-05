package service

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	msgService "kun-galgame-api/internal/message/service"
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

type CommentService struct {
	replyRepo   *repository.ReplyRepository
	commentRepo *repository.CommentRepository
	stateRepo   *userRepo.StateRepository
	userClient  *userclient.Client
	rdb         *redis.Client
	check       *gate.CheckService
	scan        *gate.ScanService
	helpers     InteractionHelpers
}

func NewCommentService(
	replyRepo *repository.ReplyRepository,
	commentRepo *repository.CommentRepository,
	stateRepo *userRepo.StateRepository,
	userClient *userclient.Client,
	rdb *redis.Client,
	check *gate.CheckService,
	scan *gate.ScanService,
) *CommentService {
	return &CommentService{
		replyRepo: replyRepo, commentRepo: commentRepo,
		stateRepo: stateRepo, userClient: userClient, rdb: rdb,
		check: check, scan: scan,
	}
}

// ──────────────────────────────────────────
// Create comment
// ──────────────────────────────────────────

// CreateComment inserts a new comment and returns it in the frontend-expected
// shape (full user + targetUser objects). Returning just a message body made
// the frontend push `undefined` into its comment list, crashing on
// `comment.user.name`.
func (s *CommentService) CreateComment(
	ctx context.Context,
	userID int,
	topicID, replyID, targetUserID int,
	parentCommentID *int,
	content string,
) (*dto.TopicCommentResponse, *errors.AppError) {
	// A nested reply must point at an existing comment ON THE SAME REPLY — guards
	// against a forged parent that would orphan the comment in another thread.
	if parentCommentID != nil {
		parent, err := s.commentRepo.FindCommentByID(*parentCommentID)
		if err != nil || parent.TopicReplyID != replyID {
			return nil, errors.ErrBadRequest("回复的评论不存在")
		}
	}

	// Synchronous Tier0 word-list gate (trust wave 2), strictly BEFORE the tx.
	// A comment's moderation text is its RAW body. deny blocks (nothing
	// persisted); hold publishes normally + logs; fail-open on any error/timeout.
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, content, &authorID)
	if decision == gate.DecisionDeny {
		return nil, errContentBlocked()
	}

	comment := &topicModel.TopicComment{
		TopicID:         topicID,
		TopicReplyID:    replyID,
		UserID:          userID,
		TargetUserID:    targetUserID,
		ParentCommentID: parentCommentID,
		Content:         content,
	}

	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.commentRepo.CreateComment(tx, comment); err != nil {
			return err
		}

		if err := recomputeTopicCounts(tx, topicID); err != nil {
			return err
		}
		// A new comment is topic activity → bump status_update_time so the topic
		// resurfaces in the last-activity feeds (mirrors reply creation). Gated on
		// the bump window in SQL: an aged-out topic is not matched, so comments no
		// longer necro-bump it (only a re-edit does).
		now := time.Now()
		if err := tx.Model(&topicModel.Topic{}).
			Where("id = ? AND created > ?", topicID, topicModel.BumpCutoff(now)).
			Update("status_update_time", now).Error; err != nil {
			return err
		}

		if userID != targetUserID {
			s.helpers.AdjustMoemoepoint(tx, targetUserID, constants.RewardReply,
				moemoepoint.ReasonContentApproved, moemoepoint.Ref("topic_reply", replyID))

			preview := truncate(content, constants.TextPreviewLength)
			s.helpers.CreateReplyMessage(tx, userID, targetUserID, "commented", preview, topicID, 0, comment.ID)
		}
		return nil
	})

	if txErr != nil {
		return nil, errors.ErrInternal("发表评论失败")
	}

	// A suspect hold publishes normally but leaves one greppable audit line;
	// then feed the RAW text into the async shadow scan, off the request path.
	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindTopicComment, "subject_id", comment.ID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindTopicComment, strconv.Itoa(comment.ID), content, int64(userID))

	// Resolve author + target in one batch via OAuth so the response carries
	// the fields the frontend TopicComment type declares.
	userMap := s.userClient.Hydrate(ctx, []int{userID, targetUserID})
	author := userMap[userID]
	target := userMap[targetUserID]

	return &dto.TopicCommentResponse{
		ID:              comment.ID,
		ReplyID:         comment.TopicReplyID,
		TopicID:         comment.TopicID,
		ParentCommentID: comment.ParentCommentID,
		User:            dto.KunUser{ID: author.ID, Name: author.Name, Avatar: author.Avatar},
		TargetUser:      dto.KunUser{ID: target.ID, Name: target.Name, Avatar: target.Avatar},
		Content:         comment.Content,
		IsLiked:         false,
		LikeCount:       0,
		Created:         comment.CreatedAt,
	}, nil
}

// ──────────────────────────────────────────
// Update (edit) comment
// ──────────────────────────────────────────

// UpdateComment lets the AUTHOR rewrite their comment's content and stamps
// `edited` (drives the "(编辑于 …)" UI). Mirrors ReplyService.UpdateReply:
// only the author may edit; admins moderate via delete, not edit.
func (s *CommentService) UpdateComment(
	ctx context.Context,
	userID int,
	req *dto.UpdateCommentRequest,
) (*dto.TopicCommentResponse, *errors.AppError) {
	comment, err := s.commentRepo.FindCommentByID(req.CommentID)
	if err != nil {
		return nil, errors.ErrNotFound("未找到该评论")
	}
	if comment.UserID != userID {
		return nil, errors.ErrForbidden("您没有权限编辑此评论")
	}

	// Synchronous word-list gate BEFORE the tx (deny blocks, hold publishes+logs,
	// fail-open). author_id is the content author (comment.UserID).
	authorID := int64(comment.UserID)
	decision, matched := s.check.Decision(ctx, req.Content, &authorID)
	if decision == gate.DecisionDeny {
		return nil, errContentBlocked()
	}

	now := time.Now()
	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		return s.commentRepo.UpdateCommentContent(tx, req.CommentID, map[string]any{
			"content": req.Content,
			"edited":  &now,
		})
	})
	if txErr != nil {
		return nil, errors.ErrInternal("编辑评论失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindTopicComment, "subject_id", req.CommentID, "author_id", comment.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindTopicComment, strconv.Itoa(req.CommentID), req.Content, int64(comment.UserID))

	// Refresh the full DTO so the FE can replace the comment in place. The
	// editor is the author, who can't like their own comment, so isLiked is
	// always false; like_count is unchanged but re-read to stay exact.
	likeCount, _ := s.commentRepo.CountCommentLikes(req.CommentID)
	userMap := s.userClient.Hydrate(ctx, []int{comment.UserID, comment.TargetUserID})
	author := userMap[comment.UserID]
	target := userMap[comment.TargetUserID]

	return &dto.TopicCommentResponse{
		ID:         comment.ID,
		ReplyID:    comment.TopicReplyID,
		TopicID:    comment.TopicID,
		User:       dto.KunUser{ID: author.ID, Name: author.Name, Avatar: author.Avatar},
		TargetUser: dto.KunUser{ID: target.ID, Name: target.Name, Avatar: target.Avatar},
		Content:    req.Content,
		IsLiked:    false,
		LikeCount:  int(likeCount),
		Created:    comment.CreatedAt,
		Edited:     &now,
	}, nil
}

// ──────────────────────────────────────────
// Toggle comment like
// ──────────────────────────────────────────

func (s *CommentService) ToggleCommentLike(ctx context.Context, userID, commentID int) *errors.AppError {
	err := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		comment, err := s.commentRepo.FindCommentByIDTx(tx, commentID)
		if err != nil {
			return err
		}
		if comment.UserID == userID {
			return gorm.ErrInvalidData
		}

		existing, findErr := s.commentRepo.FindCommentLike(tx, userID, commentID)

		if findErr == gorm.ErrRecordNotFound {
			if err := s.commentRepo.CreateCommentLike(tx, userID, commentID); err != nil {
				return err
			}
			s.helpers.AdjustMoemoepoint(tx, comment.UserID, 1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("topic_comment", commentID))

			link := msgService.BuildTopicLink(comment.TopicID, 0, comment.ID)
			preview := truncate(comment.Content, constants.TextPreviewLength)
			createDedupMessage(tx, userID, comment.UserID, "liked", preview, link)
		} else if findErr == nil {
			if err := s.commentRepo.DeleteCommentLike(tx, existing); err != nil {
				return err
			}
			s.helpers.AdjustMoemoepoint(tx, comment.UserID, -1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("topic_comment", commentID))
		} else {
			return findErr
		}
		return nil
	})

	if err == gorm.ErrInvalidData {
		return errors.ErrBadRequest("您不能给自己的评论点赞")
	}
	if err != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

// ──────────────────────────────────────────
// Delete comment
// ──────────────────────────────────────────

func (s *CommentService) DeleteComment(ctx context.Context, userID int, canModerate bool, commentID int) *errors.AppError {
	comment, err := s.commentRepo.FindCommentByID(commentID)
	if err != nil {
		return errors.ErrNotFound("未找到该评论")
	}
	if comment.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限删除此评论")
	}

	likeCount, _ := s.commentRepo.CountCommentLikes(commentID)
	penalty := 3
	if comment.UserID == userID && !canModerate {
		penalty = 3 * int(likeCount+1)
	}

	txErr := s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		state, err := s.stateRepo.LockForUpdate(tx, comment.UserID)
		if err != nil {
			return err
		}
		if state.Moemoepoint < penalty {
			return gorm.ErrCheckConstraintViolated
		}

		if err := s.commentRepo.DeleteCommentLikesForComment(tx, commentID); err != nil {
			return err
		}
		if err := s.commentRepo.DeleteCommentByID(tx, commentID); err != nil {
			return err
		}

		if err := recomputeTopicCounts(tx, comment.TopicID); err != nil {
			return err
		}

		s.helpers.AdjustMoemoepoint(tx, comment.UserID, -penalty,
			moemoepoint.ReasonContentRemoved, moemoepoint.Ref("topic_comment", commentID))
		return nil
	})

	if txErr == gorm.ErrCheckConstraintViolated {
		return errors.ErrBadRequest("萌萌点不足, 无法删除此评论")
	}
	if txErr != nil {
		return errors.ErrInternal("删除评论失败")
	}
	return nil
}

// ModerationRemove hard-deletes a topic comment for a T&S `remove` enforcement
// — no permission check, no moemoepoint penalty. Idempotent.
func (s *CommentService) ModerationRemove(commentID int) error {
	comment, err := s.commentRepo.FindCommentByID(commentID)
	if err != nil {
		return nil
	}
	return s.replyRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.commentRepo.DeleteCommentLikesForComment(tx, commentID); err != nil {
			return err
		}
		if err := s.commentRepo.DeleteCommentByID(tx, commentID); err != nil {
			return err
		}
		return recomputeTopicCounts(tx, comment.TopicID)
	})
}
