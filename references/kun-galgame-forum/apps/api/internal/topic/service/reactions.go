package service

import (
	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/topic/repository"
	"kun-galgame-api/pkg/userclient"
)

// reactionReactorIDs collects the reactor ids to hydrate. Every reaction shows a
// few avatars now (the repo's window already caps reactors per reaction), so
// there's no count gate — the FE renders up to 3 avatars + a "+N" overflow.
func reactionReactorIDs(rows []repository.ReactionRow) []int {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.UserID > 0 {
			ids = append(ids, row.UserID)
		}
	}
	return ids
}

// buildReactionSummaries turns the windowed rows for ONE target + the viewer's
// own keys + a hydrated user map into ordered DTO summaries — each carries up to
// the window's cap of reactor avatars (the rest become a "+N" on the FE).
func buildReactionSummaries(
	rows []repository.ReactionRow,
	mine []string,
	userMap map[int]userclient.User,
) []dto.ReactionSummary {
	mineSet := make(map[string]bool, len(mine))
	for _, k := range mine {
		mineSet[k] = true
	}
	idx := map[string]int{}
	out := []dto.ReactionSummary{}
	for _, row := range rows {
		i, ok := idx[row.Reaction]
		if !ok {
			i = len(out)
			idx[row.Reaction] = i
			out = append(out, dto.ReactionSummary{
				Reaction: row.Reaction,
				Count:    row.Cnt,
				Mine:     mineSet[row.Reaction],
			})
		}
		if u, ok := userMap[row.UserID]; ok {
			out[i].Reactors = append(out[i].Reactors,
				dto.KunUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar})
		}
	}
	return out
}

// replyReactorIDs is reactionReactorIDs for the batched reply rows.
func replyReactorIDs(rows []repository.ReplyReactionRow) []int {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.UserID > 0 {
			ids = append(ids, row.UserID)
		}
	}
	return ids
}

// buildRepliesReactions groups the batched reply rows by reply id and builds each
// reply's summaries against a shared (already-hydrated) user map.
func buildRepliesReactions(
	rows []repository.ReplyReactionRow,
	mineByReply map[int][]string,
	userMap map[int]userclient.User,
) map[int][]dto.ReactionSummary {
	byReply := map[int][]repository.ReactionRow{}
	for _, row := range rows {
		byReply[row.TopicReplyID] = append(byReply[row.TopicReplyID],
			repository.ReactionRow{Reaction: row.Reaction, UserID: row.UserID, Cnt: row.Cnt})
	}
	out := make(map[int][]dto.ReactionSummary, len(byReply))
	for replyID, rrows := range byReply {
		out[replyID] = buildReactionSummaries(rrows, mineByReply[replyID], userMap)
	}
	return out
}
