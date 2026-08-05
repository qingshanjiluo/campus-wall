package service

import (
	"context"
	"time"

	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/topic/dto"
	topicModel "kun-galgame-api/internal/topic/model"
	"kun-galgame-api/internal/topic/repository"
	"kun-galgame-api/pkg/userclient"
)

// ──────────────────────────────────────────
// Poll mappers
// ──────────────────────────────────────────

// buildPollResponse assembles a TopicPollResponse from a poll model and the
// associated option/voter data loaded via the repository. It does not perform
// any DB writes; callers pass in the logged-in user context via userID plus the
// caller's moderation capability. Identity for voters/creator is hydrated from
// OAuth via userclient.
// The bool return is false when the poll's creator is banned, so the caller
// drops the poll from the list (a placeholder for a merely not-found creator
// stays renderable).
func (s *PollService) buildPollResponse(ctx context.Context, poll *topicModel.TopicPoll, userID int, canModerate bool) (dto.TopicPollResponse, bool) {
	options, _ := s.pollRepo.FindOptionsByPollID(poll.ID)
	hasVoted, _ := s.pollRepo.HasUserVoted(poll.ID, userID)
	canView := canViewResults(poll, userID, canModerate, hasVoted)

	var userVotedOptionIDs map[int]bool
	if userID > 0 {
		votedIDs, _ := s.pollRepo.FindUserVoteOptionIDs(poll.ID, userID)
		userVotedOptionIDs = make(map[int]bool, len(votedIDs))
		for _, id := range votedIDs {
			userVotedOptionIDs[id] = true
		}
	}

	optionResponses := make([]dto.PollOptionResponse, len(options))
	for i, opt := range options {
		var voteCount *int
		if canView {
			vc := opt.VoteCount
			voteCount = &vc
		}
		optionResponses[i] = dto.PollOptionResponse{
			ID:        opt.ID,
			Text:      opt.Text,
			VoteCount: voteCount,
			IsVoted:   userVotedOptionIDs[opt.ID],
		}
	}

	var voters []dto.KunUser
	var votersCount int
	var totalVoteCount *int
	if canView {
		if !poll.IsAnonymous {
			voterIDs, _ := s.pollRepo.FindDistinctVoterIDs(poll.ID, 5)
			vmap := s.userClient.Hydrate(ctx, voterIDs)
			voters = make([]dto.KunUser, 0, len(voterIDs))
			for _, id := range voterIDs {
				u := vmap[id]
				if !userclient.IsRenderable(u) {
					continue
				}
				voters = append(voters, dto.KunUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar})
			}
		}
		vc, _ := s.pollRepo.CountDistinctVoters(poll.ID)
		votersCount = vc
		tc, _ := s.pollRepo.CountTotalVotes(poll.ID)
		totalVoteCount = &tc
	}
	if voters == nil {
		voters = []dto.KunUser{}
	}

	creatorU, _, _ := s.userClient.User(ctx, poll.UserID)
	if creatorU.ID == 0 {
		creatorU = userclient.Placeholder(poll.UserID)
	}
	// A banned creator's poll is hidden entirely (its content is theirs).
	if !userclient.IsRenderable(creatorU) {
		return dto.TopicPollResponse{}, false
	}

	return dto.TopicPollResponse{
		ID: poll.ID, Title: poll.Title, Description: poll.Description,
		MinChoice: poll.MinChoice, MaxChoice: poll.MaxChoice,
		Deadline: poll.Deadline, Type: poll.Type, Status: poll.Status,
		ResultVisibility: poll.ResultVisibility,
		IsAnonymous:      poll.IsAnonymous, CanChangeVote: poll.CanChangeVote,
		TopicID: poll.TopicID, Created: poll.CreatedAt, Updated: poll.UpdatedAt,
		User:     dto.KunUser{ID: creatorU.ID, Name: creatorU.Name, Avatar: creatorU.Avatar},
		Options:  optionResponses,
		HasVoted: hasVoted, Voters: voters,
		VotersCount: votersCount, VoteCount: totalVoteCount,
	}, true
}

// canViewResults returns true if the caller is allowed to see vote counts /
// voter identities according to the poll's result_visibility setting.
func canViewResults(poll *topicModel.TopicPoll, userID int, canModerate, hasVoted bool) bool {
	if userID == poll.UserID || canModerate {
		return true
	}
	isPollFinished := poll.Status == "closed" ||
		(poll.Deadline != nil && time.Now().After(*poll.Deadline))

	switch poll.ResultVisibility {
	case "always":
		return true
	case "after_vote":
		return hasVoted
	case "after_deadline":
		return isPollFinished
	default:
		return false
	}
}

// ──────────────────────────────────────────
// Reply mappers
// ──────────────────────────────────────────

// buildReplyResponses turns a batch of ReplyRow into TopicReplyResponse DTOs.
// Fetches targets/comments/like-status via the repository in bulk. Identity
// (name/avatar/moemoepoint) is hydrated from OAuth+kungal_user_state since
// the repo no longer joins on the user table.
func (s *ReplyService) buildReplyResponses(
	ctx context.Context,
	rows []repository.ReplyRow,
	topic *topicModel.Topic,
	userInfo *middleware.UserInfo,
) []dto.TopicReplyResponse {
	if len(rows) == 0 {
		return nil
	}

	replyIDs := make([]int, len(rows))
	for i, r := range rows {
		replyIDs[i] = r.ID
	}

	commentMap, _ := s.commentRepo.FindCommentsByReplyIDs(replyIDs)
	reactionRows, _ := s.replyRepo.GetRepliesReactions(replyIDs)

	var likeMap, dislikeMap map[int]bool
	var commentLikeMap map[int]bool
	var mineReactions map[int][]string
	if userInfo != nil {
		likeMap, _ = s.replyRepo.FindReplyLikeStatus(userInfo.ID, replyIDs)
		dislikeMap, _ = s.replyRepo.FindReplyDislikeStatus(userInfo.ID, replyIDs)
		mineReactions, _ = s.replyRepo.GetUserRepliesReactions(replyIDs, userInfo.ID)

		var commentIDs []int
		for _, comments := range commentMap {
			for _, c := range comments {
				commentIDs = append(commentIDs, c.ID)
			}
		}
		commentLikeMap, _ = s.commentRepo.FindCommentLikeStatus(userInfo.ID, commentIDs)
	}

	// Collect every userID we'll render: reply authors + target authors + comment
	// authors + comment target authors. Hydrate in one batch.
	uidSet := make(map[int]struct{})
	for _, r := range rows {
		uidSet[r.UserID] = struct{}{}
		for _, id := range markdown.ExtractMentionIDs(r.Content) {
			uidSet[id] = struct{}{}
		}
	}
	for _, cs := range commentMap {
		for _, c := range cs {
			uidSet[c.UserID] = struct{}{}
			if c.TargetUserID > 0 {
				uidSet[c.TargetUserID] = struct{}{}
			}
		}
	}
	for _, id := range replyReactorIDs(reactionRows) {
		uidSet[id] = struct{}{}
	}
	uids := make([]int, 0, len(uidSet))
	for id := range uidSet {
		if id > 0 {
			uids = append(uids, id)
		}
	}
	userMap := s.userClient.Hydrate(ctx, uids)
	repliesReactions := buildRepliesReactions(reactionRows, mineReactions, userMap)

	// moemoepoint comes from kungal_user_state, not OAuth.
	moeMap := make(map[int]int, len(rows))
	for _, r := range rows {
		if _, seen := moeMap[r.UserID]; seen {
			continue
		}
		if state, _ := s.stateRepo.FindByID(r.UserID); state != nil {
			moeMap[r.UserID] = state.Moemoepoint
		}
	}

	kunUser := func(id int) dto.KunUser {
		u := userMap[id]
		return dto.KunUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
	}

	// Mention name resolution: render the body, then swap each @mention's text to
	// the author's CURRENT name (mentioned ids were batched into userMap above),
	// so a renamed user shows their new name without the stored post changing.
	mentionNames := make(map[int]string, len(userMap))
	for id, u := range userMap {
		mentionNames[id] = u.Name
	}
	renderContent := func(c string) string {
		return markdown.ResolveMentionNames(markdown.Render(c), mentionNames)
	}

	responses := make([]dto.TopicReplyResponse, 0, len(rows))
	for _, r := range rows {
		// Drop banned authors entirely.
		if u, ok := userMap[r.UserID]; ok && !userclient.IsRenderable(u) {
			continue
		}
		var comments []dto.TopicCommentResponse
		if cs, ok := commentMap[r.ID]; ok {
			for _, c := range cs {
				isLiked := false
				if commentLikeMap != nil {
					isLiked = commentLikeMap[c.ID]
				}
				// Banned comment author: tombstone the node (placeholder
				// identity + blank content) rather than drop it — comments
				// nest via ParentCommentID, so removing one with child
				// comments would orphan them in the FE tree.
				commentUser := kunUser(c.UserID)
				content := c.Content
				if cu, ok := userMap[c.UserID]; ok && !userclient.IsRenderable(cu) {
					ph := userclient.Placeholder(c.UserID)
					commentUser = dto.KunUser{ID: ph.ID, Name: ph.Name, Avatar: ph.Avatar}
					content = ""
				}
				comments = append(comments, dto.TopicCommentResponse{
					ID:              c.ID,
					ReplyID:         c.TopicReplyID,
					TopicID:         c.TopicID,
					ParentCommentID: c.ParentCommentID,
					User:            commentUser,
					TargetUser:      kunUser(c.TargetUserID),
					Content:         content,
					IsLiked:         isLiked,
					LikeCount:       c.LikeCount,
					Created:         c.CreatedAt,
					Edited:          c.Edited,
				})
			}
		}
		if comments == nil {
			comments = []dto.TopicCommentResponse{}
		}

		isPinned := topic != nil && topic.PinnedReplyID != nil && *topic.PinnedReplyID == r.ID
		isBestAnswer := topic != nil && topic.BestAnswerID != nil && *topic.BestAnswerID == r.ID

		author := userMap[r.UserID]
		responses = append(responses, dto.TopicReplyResponse{
			ID:      r.ID,
			TopicID: r.TopicID,
			Floor:   r.Floor,
			User: dto.KunUserWithMoemoepoint{
				ID:          author.ID,
				Name:        author.Name,
				Avatar:      author.Avatar,
				Moemoepoint: moeMap[r.UserID],
			},
			Edited:          r.Edited,
			ContentMarkdown: r.Content,
			ContentHtml:     renderContent(r.Content),
			LikeCount:       r.LikeCount,
			IsLiked:         likeMap[r.ID],
			DislikeCount:    r.DislikeCount,
			IsDisliked:      dislikeMap[r.ID],
			Reactions:       repliesReactions[r.ID],
			Comments:        comments,
			IsPinned:        isPinned,
			IsBestAnswer:    isBestAnswer,
			Created:         r.CreatedAt,
		})
	}
	return responses
}

// ──────────────────────────────────────────
// Topic mappers
// ──────────────────────────────────────────

// toTopicCard maps a TopicCardRow with its section slice to a TopicCard DTO.
// Shared by GetList and GetResourceList.
func toTopicCard(r repository.TopicCardRow, sections []string, isPollTopic bool) dto.TopicCard {
	if sections == nil {
		sections = []string{}
	}
	covers := []string(r.CoverImages)
	if covers == nil {
		covers = []string{}
	}
	return dto.TopicCard{
		ID:          r.ID,
		Title:       r.Title,
		View:        r.View,
		Sections:    sections,
		CoverImages: covers,
		// Reserve each cover's aspect ratio (no CLS) + blur-up on the FE.
		CoverImageMeta: markdown.ResolveContentImageMeta(covers),
		User: dto.KunUser{
			ID:     r.UserID,
			Name:   r.UserName,
			Avatar: r.UserAvatar,
		},
		Status:           r.Status,
		HasBestAnswer:    r.BestAnswerID != nil,
		IsPollTopic:      isPollTopic,
		IsNSFW:           r.IsNSFW,
		LikeCount:        r.LikeCount,
		ReplyCount:       r.ReplyCount,
		CommentCount:     r.CommentCount,
		StatusUpdateTime: r.StatusUpdateTime,
		Created:          r.Created,
		UpvoteTime:       r.UpvoteTime,
	}
}
