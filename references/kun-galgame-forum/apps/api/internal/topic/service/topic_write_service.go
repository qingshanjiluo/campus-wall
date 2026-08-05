package service

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/infrastructure/markdown"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/internal/topic/dto"
	topicModel "kun-galgame-api/internal/topic/model"
	"kun-galgame-api/internal/topic/repository"
	"kun-galgame-api/internal/trust/gate"
	userRepo "kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/errors"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type TopicWriteService struct {
	topicRepo    *repository.TopicRepository
	taxonomyRepo *repository.TopicTaxonomyRepository
	replyRepo    *repository.ReplyRepository
	stateRepo    *userRepo.StateRepository
	rdb          *redis.Client
	notifier     msgService.Notifier
	check        *gate.CheckService
	scan         *gate.ScanService
	helpers      InteractionHelpers
}

func NewTopicWriteService(
	topicRepo *repository.TopicRepository,
	taxonomyRepo *repository.TopicTaxonomyRepository,
	replyRepo *repository.ReplyRepository,
	stateRepo *userRepo.StateRepository,
	rdb *redis.Client,
	notifier msgService.Notifier,
	check *gate.CheckService,
	scan *gate.ScanService,
) *TopicWriteService {
	return &TopicWriteService{
		topicRepo:    topicRepo,
		taxonomyRepo: taxonomyRepo,
		replyRepo:    replyRepo,
		stateRepo:    stateRepo,
		rdb:          rdb,
		notifier:     notifier,
		check:        check,
		scan:         scan,
	}
}

// topicModerationText composes the RAW text the trust check + scan see for a
// topic: title + blank line + body (onboarding §3.1 convention for titled
// content). Sent RAW, never rendered HTML.
func topicModerationText(title, content string) string {
	return title + "\n\n" + content
}

// errContentBlocked is the 422-class, user-visible response when the synchronous
// trust word-list gate DENIES a write (a banned term). Nothing is persisted.
// Delegates to gate.ErrContentBlocked so every write surface (wave 1 + 2) shares
// one copy of the deliberately vague message.
func errContentBlocked() *errors.AppError {
	return gate.ErrContentBlocked()
}

// anyConsumeSection reports whether any section name is a paid
// (moemoepoint-consuming) one — see constants.TopicSectionConsume.
func anyConsumeSection(sections []string) bool {
	for _, sec := range sections {
		if constants.TopicSectionConsume[sec] {
			return true
		}
	}
	return false
}

// topicSectionFootprint is a topic's moemoepoint outcome given whether it sits
// in a paid section, mirroring Create: a paid section is a net deduction
// (−CostConsumeSection) and forfeits the +RewardCreateTopic base reward a free
// section earns. Update applies the DIFFERENCE between the old and new footprint
// so moving a topic between section tiers leaves the author exactly where a
// fresh post in the new tier would — charging on move-in, refunding on move-out.
func topicSectionFootprint(consume bool) int {
	if consume {
		return -constants.CostConsumeSection
	}
	return constants.RewardCreateTopic
}

// coverImageTokenRe matches a single /image/<64hex> content token — the only
// shape a topic cover may take (see model.ImageTokens / migration 029).
var coverImageTokenRe = regexp.MustCompile(`^/image/[0-9a-f]{64}$`)

// normalizeCoverImages validates the incoming cover tokens, drops duplicates
// (keeping first-seen order), and returns them ready to persist. A malformed
// token or a list longer than 9 is a bad request. Empty in → nil (no covers).
func normalizeCoverImages(in []string) (topicModel.ImageTokens, *errors.AppError) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > 9 {
		return nil, errors.ErrBadRequest("封面图最多 9 张")
	}
	seen := make(map[string]struct{}, len(in))
	out := make(topicModel.ImageTokens, 0, len(in))
	for _, tk := range in {
		if !coverImageTokenRe.MatchString(tk) {
			return nil, errors.ErrBadRequest("封面图格式不正确")
		}
		if _, dup := seen[tk]; dup {
			continue
		}
		seen[tk] = struct{}{}
		out = append(out, tk)
	}
	return out, nil
}

// ──────────────────────────────────────────
// Create — all checks inside transaction
// ──────────────────────────────────────────

func (s *TopicWriteService) Create(
	ctx context.Context,
	userID int,
	req *dto.CreateTopicRequest,
) (int, *errors.AppError) {
	hasConsumeSection := anyConsumeSection(req.Sections)

	// No banner set but the body has images → auto-pick up to 9 of them as the
	// cover (same cap as a self-uploaded banner). Publish-time only; re-edit keeps
	// whatever the author has.
	coverInput := req.CoverImages
	if len(coverInput) == 0 {
		coverInput = markdown.ExtractContentImages(req.Content, 9)
	}
	covers, coverErr := normalizeCoverImages(coverInput)
	if coverErr != nil {
		return 0, coverErr
	}

	// Synchronous Tier0 word-list gate (trust wave 1), strictly BEFORE the tx —
	// the HTTP check never runs inside a transaction. deny blocks the write
	// outright (nothing persisted); hold publishes normally + logs (below). A
	// disabled gate / any error / timeout is a fail-open allow.
	moderationText := topicModerationText(req.Title, req.Content)
	authorID := int64(userID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return 0, errContentBlocked()
	}

	var newTopicID int

	err := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		state, err := s.stateRepo.LockForUpdate(tx, userID)
		if err != nil {
			return err
		}

		todayCount, err := s.topicRepo.CountTodayTopicsByUser(tx, userID)
		if err != nil {
			return err
		}
		dailyLimit := int64(state.Moemoepoint/10 + 1)
		if todayCount >= dailyLimit {
			return gorm.ErrInvalidData
		}

		if hasConsumeSection && state.Moemoepoint < constants.CostConsumeSection {
			return gorm.ErrInvalidData
		}

		topic := &topicModel.Topic{
			Title:       req.Title,
			Content:     req.Content,
			Category:    req.Category,
			IsNSFW:      req.IsNSFW,
			UserID:      userID,
			CoverImages: covers,
		}
		if err := s.topicRepo.CreateTopic(tx, topic); err != nil {
			return err
		}
		newTopicID = topic.ID

		sections, err := s.taxonomyRepo.FindSectionsByNamesTx(tx, req.Sections)
		if err != nil {
			return err
		}
		for _, sec := range sections {
			if err := s.taxonomyRepo.CreateSectionRelation(tx, topic.ID, sec.ID); err != nil {
				return err
			}
		}

		// Posting in a paid section is a net deduction (cost > reward); a free
		// section earns the base reward. topicSectionFootprint owns this rule so
		// Update's section-change adjustment stays in lock-step.
		pointsDelta := topicSectionFootprint(hasConsumeSection)
		mpReason := moemoepoint.ReasonContentApproved
		if hasConsumeSection {
			mpReason = moemoepoint.ReasonContentRemoved
		}
		s.helpers.AdjustMoemoepoint(tx, userID, pointsDelta, mpReason, moemoepoint.Ref("topic", topic.ID))
		// @mentions in the topic body → "mentioned" notifications (deduped, self
		// skip). Topic-level target (0/0) — links to the topic root.
		s.helpers.NotifyMentions(tx, userID, topic.ID, 0, 0, req.Content)
		return nil
	})

	if err != nil {
		if err == gorm.ErrInvalidData {
			if hasConsumeSection {
				return 0, errors.ErrBadRequest("您的萌萌点不足, 无法发布此类型话题")
			}
			return 0, errors.ErrBadRequest("您今日发布的话题已达上限")
		}
		return 0, errors.ErrInternal("创建话题失败")
	}

	// Published. A suspect hold publishes normally but leaves one greppable audit
	// line (v1 builds no local queue entry — scan records it). Then feed the RAW
	// text into the async shadow scan, off the request path, after commit.
	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindTopic, "subject_id", newTopicID, "author_id", userID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindTopic, strconv.Itoa(newTopicID), moderationText, int64(userID))

	return newTopicID, nil
}

// ──────────────────────────────────────────
// Update
// ──────────────────────────────────────────

func (s *TopicWriteService) Update(
	ctx context.Context,
	userID int,
	canModerate bool,
	topicID int,
	req *dto.UpdateTopicRequest,
) *errors.AppError {
	topic, err := s.topicRepo.FindByID(topicID)
	if err != nil {
		return errors.ErrNotFound("未找到该话题")
	}
	if topic.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限编辑此话题")
	}

	// Snapshot the current sections BEFORE the replace below so we can tell
	// whether this edit moves the topic between the free/paid section tiers.
	// Create only charged at publish time; changing the section afterwards
	// (e.g. an admin moving someone else's topic into a paid section) must
	// charge/refund the AUTHOR too — otherwise a paid section is free if you
	// publish elsewhere first and get moved in.
	oldSections, err := s.taxonomyRepo.FindSectionNamesByTopicID(topicID)
	if err != nil {
		return errors.ErrInternal("更新话题失败")
	}
	oldConsume := anyConsumeSection(oldSections)
	newConsume := anyConsumeSection(req.Sections)

	covers, coverErr := normalizeCoverImages(req.CoverImages)
	if coverErr != nil {
		return coverErr
	}

	// Synchronous word-list gate BEFORE the tx (deny blocks, hold publishes+logs,
	// fail-open). author_id is the content author (topic.UserID), not the editor.
	moderationText := topicModerationText(req.Title, req.Content)
	authorID := int64(topic.UserID)
	decision, matched := s.check.Decision(ctx, moderationText, &authorID)
	if decision == gate.DecisionDeny {
		return errContentBlocked()
	}

	now := time.Now()
	txErr := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.topicRepo.UpdateTopicFields(tx, topicID, map[string]any{
			"title":              req.Title,
			"content":            req.Content,
			"category":           req.Category,
			"is_nsfw":            req.IsNSFW,
			"cover_images":       covers,
			"edited":             &now,
			"status_update_time": now,
		}); err != nil {
			return err
		}

		sections, err := s.taxonomyRepo.FindSectionsByNamesTx(tx, req.Sections)
		if err != nil {
			return err
		}
		sectionIDs := make([]int, len(sections))
		for i, sec := range sections {
			sectionIDs[i] = sec.ID
		}
		if err := s.taxonomyRepo.ReplaceSectionRelations(tx, topicID, sectionIDs); err != nil {
			return err
		}

		// Charge/refund the AUTHOR (not the editor) when this edit crosses the
		// free/paid section tier. delta is the footprint difference; its sign
		// picks the audit reason, mirroring Create. No-op when the tier is
		// unchanged. Like Create, the award is async/best-effort and not gated
		// on the author's balance here — an admin moderating a topic shouldn't
		// be blocked by, or have to reason about, someone else's wallet.
		if delta := topicSectionFootprint(newConsume) - topicSectionFootprint(oldConsume); delta != 0 {
			mpReason := moemoepoint.ReasonContentApproved
			if delta < 0 {
				mpReason = moemoepoint.ReasonContentRemoved
			}
			s.helpers.AdjustMoemoepoint(tx, topic.UserID, delta, mpReason, moemoepoint.Ref("topic", topicID))
		}

		// @mentions in the edited topic body → notify newly mentioned users
		// (deduped, so existing mentions aren't re-notified on edit). Topic-level.
		s.helpers.NotifyMentions(tx, userID, topicID, 0, 0, req.Content)

		return nil
	})

	if txErr != nil {
		return errors.ErrInternal("更新话题失败")
	}

	if decision == gate.DecisionHold {
		slog.Info("trust check hold", "subject_kind", gate.SubjectKindTopic, "subject_id", topicID, "author_id", topic.UserID, "matched", matched)
	}
	s.scan.ScanBg(gate.SubjectKindTopic, strconv.Itoa(topicID), moderationText, int64(topic.UserID))

	return nil
}

// ──────────────────────────────────────────
// Interactions — all checks inside transaction
// ──────────────────────────────────────────

// reactionKeys is the allowlist of valid reaction keys: the two effectful ones
// (like/dislike) + the emoji set (kept in sync with the FE picker / assets).
var reactionKeys = map[string]bool{
	"like": true, "dislike": true,
	"heart": true, "fire": true, "party": true, "love": true,
	"clap": true, "thinking": true, "mindblown": true, "scream": true,
	"cry": true, "pray": true, "eyes": true, "hundred": true,
	"partyface": true, "starstruck": true,
	"angry": true, "anxious": true, "banana": true, "eyebrow": true,
	"voltage": true, "hotdog": true, "hot": true, "sob": true,
	"moai": true, "newmoon": true, "police": true, "pouting": true,
	"salute": true, "shrimp": true, "halo": true, "sunglasses": true,
	"whale": true,
}

// ToggleLike / ToggleDislike are kept as thin aliases (the legacy endpoints still
// call them) — both now route through the unified reaction path.
func (s *TopicWriteService) ToggleLike(ctx context.Context, userID, topicID int) *errors.AppError {
	return s.ToggleReaction(ctx, userID, topicID, "like")
}

func (s *TopicWriteService) ToggleDislike(ctx context.Context, userID, topicID int) *errors.AppError {
	return s.ToggleReaction(ctx, userID, topicID, "dislike")
}

// ToggleReaction adds/removes a reaction on a topic. like/dislike are effectful
// (like → ±1 moemoepoint to the owner + a "liked" notification) and mutually
// exclusive; emoji reactions are plain. Only 'like' is blocked on one's own
// topic (it grants the owner moemoepoint). like/dislike counts stay denormalized
// on the topic row.
func (s *TopicWriteService) ToggleReaction(ctx context.Context, userID, topicID int, reaction string) *errors.AppError {
	if !reactionKeys[reaction] {
		return errors.ErrBadRequest("无效的 reaction")
	}
	err := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		topic, err := s.topicRepo.FindByIDTx(tx, topicID)
		if err != nil {
			return err
		}
		if topic.Status == 1 {
			return gorm.ErrRecordNotFound
		}
		if reaction == "like" && topic.UserID == userID {
			return gorm.ErrInvalidData
		}

		has, err := s.topicRepo.HasReaction(tx, topicID, userID, reaction)
		if err != nil {
			return err
		}
		if has {
			if err := s.topicRepo.RemoveReaction(tx, topicID, userID, reaction); err != nil {
				return err
			}
			switch reaction {
			case "like":
				if err := s.topicRepo.AdjustLikeCount(tx, topicID, -1); err != nil {
					return err
				}
				s.helpers.AdjustMoemoepoint(tx, topic.UserID, -1,
					moemoepoint.ReasonLiked, moemoepoint.Ref("topic", topicID))
			case "dislike":
				if err := s.topicRepo.AdjustDislikeCount(tx, topicID, -1); err != nil {
					return err
				}
			}
			return nil
		}

		switch reaction {
		case "like":
			if err := s.clearTopicReaction(tx, topicID, userID, "dislike", topic.UserID); err != nil {
				return err
			}
			if err := s.topicRepo.AddReaction(tx, topicID, userID, "like"); err != nil {
				return err
			}
			if err := s.topicRepo.AdjustLikeCount(tx, topicID, 1); err != nil {
				return err
			}
			s.helpers.AdjustMoemoepoint(tx, topic.UserID, 1,
				moemoepoint.ReasonLiked, moemoepoint.Ref("topic", topicID))
			s.helpers.CreateTopicMessageWithContent(tx, userID, topic.UserID, "liked",
				truncate(topic.Title, constants.TextPreviewLength), topicID, 0, 0)
		case "dislike":
			if err := s.clearTopicReaction(tx, topicID, userID, "like", topic.UserID); err != nil {
				return err
			}
			if err := s.topicRepo.AddReaction(tx, topicID, userID, "dislike"); err != nil {
				return err
			}
			if err := s.topicRepo.AdjustDislikeCount(tx, topicID, 1); err != nil {
				return err
			}
		default:
			if err := s.topicRepo.AddReaction(tx, topicID, userID, reaction); err != nil {
				return err
			}
		}
		return nil
	})

	switch err {
	case nil:
		return nil
	case gorm.ErrRecordNotFound:
		return errors.ErrNotFound("未找到该话题")
	case gorm.ErrInvalidData:
		return errors.ErrBadRequest("您不能给自己点赞")
	default:
		return errors.ErrInternal("操作失败")
	}
}

// clearTopicReaction removes the user's `reaction` if present (for like⇄dislike
// exclusion), reversing the like count + moemoepoint when it was a 'like'.
func (s *TopicWriteService) clearTopicReaction(tx *gorm.DB, topicID, userID int, reaction string, ownerID int) error {
	has, err := s.topicRepo.HasReaction(tx, topicID, userID, reaction)
	if err != nil || !has {
		return err
	}
	if err := s.topicRepo.RemoveReaction(tx, topicID, userID, reaction); err != nil {
		return err
	}
	switch reaction {
	case "like":
		if err := s.topicRepo.AdjustLikeCount(tx, topicID, -1); err != nil {
			return err
		}
		s.helpers.AdjustMoemoepoint(tx, ownerID, -1,
			moemoepoint.ReasonLiked, moemoepoint.Ref("topic", topicID))
	case "dislike":
		if err := s.topicRepo.AdjustDislikeCount(tx, topicID, -1); err != nil {
			return err
		}
	}
	return nil
}

func (s *TopicWriteService) Upvote(ctx context.Context, userID, topicID int, description string) *errors.AppError {
	description = truncate(description, 30)
	err := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		topic, err := s.topicRepo.FindByIDTx(tx, topicID)
		if err != nil {
			return err
		}
		if topic.Status == 1 {
			return gorm.ErrRecordNotFound
		}
		if topic.UserID == userID {
			return gorm.ErrInvalidData
		}

		state, err := s.stateRepo.LockForUpdate(tx, userID)
		if err != nil {
			return err
		}
		// Repeat upvotes ARE allowed: a topic can be pushed again and again. Each
		// push costs the sender afresh (the FOR UPDATE lock serializes the balance)
		// and credits the owner again — self-limited by the 10-萌萌点 cost.
		if state.Moemoepoint < constants.CostUpvoteSender {
			return gorm.ErrCheckConstraintViolated
		}

		now := time.Now()

		if err := s.topicRepo.CreateTopicUpvote(tx, userID, topicID, description); err != nil {
			return err
		}
		if err := s.topicRepo.ApplyUpvoteCountAndTime(tx, topicID, now); err != nil {
			return err
		}

		// Distinct ref-kind ("topic_upvote") so the moemoepoint ledger renders
		// 推话题消耗 / 话题被推荐 — NOT 话题被移除 / 话题被采纳, which the bare "topic"
		// kind composes. The reason enum is OAuth's (content_removed is the only
		// debit reason a downstream may use), so the ref is what disambiguates.
		s.helpers.AdjustMoemoepoint(tx, userID, -constants.CostUpvoteSender,
			moemoepoint.ReasonContentRemoved, moemoepoint.Ref("topic_upvote", topicID))
		s.helpers.AdjustMoemoepoint(tx, topic.UserID, constants.RewardUpvoteOwner,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("topic_upvote", topicID))
		s.helpers.CreateTopicMessageWithContent(tx, userID, topic.UserID, "upvoted",
			truncate(topic.Title, constants.TextPreviewLength), topicID, 0, 0)
		return nil
	})

	if err == gorm.ErrRecordNotFound {
		return errors.ErrNotFound("未找到该话题")
	}
	if err == gorm.ErrInvalidData {
		return errors.ErrBadRequest("您不能推自己的话题")
	}
	if err == gorm.ErrCheckConstraintViolated {
		return errors.ErrBadRequest("萌萌点不足, 推话题需要 10 萌萌点")
	}
	if err != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

func (s *TopicWriteService) ToggleFavorite(ctx context.Context, userID, topicID int) *errors.AppError {
	err := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		topic, err := s.topicRepo.FindByIDTx(tx, topicID)
		if err != nil {
			return err
		}
		if topic.Status == 1 {
			return gorm.ErrRecordNotFound
		}

		existing, findErr := s.topicRepo.FindTopicFavorite(tx, userID, topicID)

		if findErr == gorm.ErrRecordNotFound {
			if err := s.topicRepo.CreateTopicFavorite(tx, userID, topicID); err != nil {
				return err
			}
			if err := s.topicRepo.AdjustFavoriteCount(tx, topicID, 1); err != nil {
				return err
			}
			if userID != topic.UserID {
				s.helpers.AdjustMoemoepoint(tx, topic.UserID, 1,
					moemoepoint.ReasonLiked, moemoepoint.Ref("topic", topicID))
				s.helpers.CreateTopicMessageWithContent(tx, userID, topic.UserID, "favorite",
					truncate(topic.Title, constants.TextPreviewLength), topicID, 0, 0)
			}
		} else if findErr == nil {
			if err := s.topicRepo.DeleteTopicFavorite(tx, existing); err != nil {
				return err
			}
			if err := s.topicRepo.AdjustFavoriteCount(tx, topicID, -1); err != nil {
				return err
			}
			if userID != topic.UserID {
				s.helpers.AdjustMoemoepoint(tx, topic.UserID, -1,
					moemoepoint.ReasonLiked, moemoepoint.Ref("topic", topicID))
			}
		} else {
			return findErr
		}
		return nil
	})

	if err == gorm.ErrRecordNotFound {
		return errors.ErrNotFound("未找到该话题")
	}
	if err != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

func (s *TopicWriteService) ToggleHide(ctx context.Context, userID int, canModerate bool, topicID int) *errors.AppError {
	topic, err := s.topicRepo.FindByID(topicID)
	if err != nil {
		return errors.ErrNotFound("未找到该话题")
	}
	if topic.UserID != userID && !canModerate {
		return errors.ErrForbidden("您没有权限操作此话题")
	}

	newStatus := 1
	if topic.Status == 1 {
		newStatus = 0
	}
	if err := s.topicRepo.UpdateFields(topicID, map[string]any{"status": newStatus}); err != nil {
		return errors.ErrInternal("操作失败")
	}
	return nil
}

// SetBestAnswer toggles the best answer for a topic: if the given reply is
// already the current best answer it is cleared, otherwise it becomes the
// best answer. The reply author's moemoepoint is adjusted by ±7 to match
// the legacy Nitro behavior.
func (s *TopicWriteService) SetBestAnswer(ctx context.Context, userID int, canModerate bool, topicID, replyID int) *errors.AppError {
	topic, err := s.topicRepo.FindByID(topicID)
	if err != nil {
		return errors.ErrNotFound("未找到该话题")
	}
	if topic.UserID != userID && !canModerate {
		return errors.ErrForbidden("只有话题作者或管理员可以设置最佳回答")
	}

	var reply topicModel.TopicReply
	if err := s.topicRepo.DB().First(&reply, replyID).Error; err != nil {
		return errors.ErrNotFound("未找到该回复")
	}
	if reply.TopicID != topicID {
		return errors.ErrBadRequest("该回复不属于此话题")
	}

	isCurrentBest := topic.BestAnswerID != nil && *topic.BestAnswerID == replyID
	delta := 7
	if isCurrentBest {
		delta = -7
	}

	txErr := s.topicRepo.DB().Transaction(func(tx *gorm.DB) error {
		if isCurrentBest {
			if err := tx.Model(&topicModel.Topic{}).Where("id = ?", topicID).
				Update("best_answer_id", nil).Error; err != nil {
				return err
			}
		} else {
			// best_answer_id always sets; only the status_update_time bump is gated
			// on the bump window (necro-bump prevention) via an in-SQL CASE so the
			// pair stays one race-free statement.
			now := time.Now()
			if err := tx.Model(&topicModel.Topic{}).Where("id = ?", topicID).
				Updates(map[string]any{
					"best_answer_id": &replyID,
					"status_update_time": gorm.Expr(
						"CASE WHEN created > ? THEN ? ELSE status_update_time END",
						topicModel.BumpCutoff(now), now),
				}).Error; err != nil {
				return err
			}
		}
		// Use the helper (kungal_user_state) instead of raw `UPDATE "user"
		// SET moemoepoint = ...` — migration 007 dropped that column from
		// the identity table, so the legacy SQL was PG-erroring out and
		// silently rolling back the entire set-best-answer transaction.
		bestReason := moemoepoint.ReasonContentApproved
		if delta < 0 {
			bestReason = moemoepoint.ReasonContentRemoved
		}
		s.helpers.AdjustMoemoepoint(tx, reply.UserID, delta, bestReason, moemoepoint.Ref("topic_reply", reply.ID))
		// Only notify on set (not on clear) — matches legacy Nitro.
		if !isCurrentBest {
			return s.notifier.Emit(tx, msgService.Spec{
				SenderID:   userID,
				ReceiverID: reply.UserID,
				Kind:       msgService.NotifySolution,
				Content:    replyPlainPreview(reply),
				TopicID:    topicID,
				ReplyFloor: reply.Floor,
			})
		}
		return nil
	})
	if txErr != nil {
		return errors.ErrInternal("设置最佳回答失败")
	}
	return nil
}
