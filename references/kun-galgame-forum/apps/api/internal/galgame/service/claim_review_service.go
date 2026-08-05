package service

import (
	"context"
	"net/url"
	"strconv"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/errors"
)

// ClaimReviewService is the moderation side of the submission lifecycle: the
// queue of entries awaiting review, and the four verdicts.
//
// It replaces the wiki's admin message queue, and the two are not the same
// object. The wiki queue listed MESSAGES — a log of things that had happened,
// which a moderator read as a worklist by convention. This lists CLAIMS in the
// pending state, which is the worklist itself: an entry leaves it because its
// state moved, not because someone marked a row read. That is also why there is
// no "mark as handled" here and no way for the queue to disagree with reality.
//
// Authority: the four verdicts require `catalog.claim.review`, a permission
// granted to moderator and up so that the wiki's moderation staffing carries
// over unchanged. The forum's own RequireModerator gate stays as the ENTRY
// check; the registry re-checks the asserted actor, so this BFF holds no policy.
type ClaimReviewService struct {
	galgameClient *client.GalgameClient
	catalog       *catalogclient.Client
}

func NewClaimReviewService(
	galgameClient *client.GalgameClient,
	catalog *catalogclient.Client,
) *ClaimReviewService {
	return &ClaimReviewService{galgameClient: galgameClient, catalog: catalog}
}

// pendingQueueLimit is the queue page size.
const pendingQueueLimit = 30

// PendingQueuePage is the wire envelope of the review queue.
//
// It exists because client.CatalogWorksPage carries NO json tags — it is an
// internal projection of the catalog face, never meant to be a response body,
// and marshalling it directly ships PascalCase keys ("Items"/"NextCursor").
// The page reads data.items, so that shape was a TypeError on every load.
// Every other consumer of that struct already maps it (see the calendar
// service); this face is the one that did not.
type PendingQueuePage struct {
	Items      []client.CatalogWorkListItem `json:"items"`
	NextCursor string                       `json:"next_cursor"`
}

// PendingQueue lists kungal's submissions awaiting review, newest first.
//
// It reads the LIVE works list rather than the search face on purpose: the
// search index reflects a claim-state change only at the next reindex, and a
// moderation queue that shows entries somebody already approved an hour ago is
// worse than no queue. `site=` keeps another product's backlog out of kungal's
// queue — without it the page would have to post-filter, which would make its
// total and its cursor disagree with what it shows.
func (s *ClaimReviewService) PendingQueue(
	ctx context.Context,
	query url.Values,
) (*PendingQueuePage, *errors.AppError) {
	q := url.Values{
		"claimed":     {"true"},
		"claim_state": {catalogclient.ClaimStatePending},
		"site":        {submissionSite},
		"sort":        {"updated"},
		"limit":       {strconv.Itoa(atoiOr(query.Get("limit"), pendingQueueLimit))},
		"include":     {wizardSearchInclude},
	}
	if cursor := query.Get("cursor"); cursor != "" {
		q.Set("cursor", cursor)
	}
	// The age gate is open: a moderation queue that cannot see r18 submissions
	// cannot moderate 94% of them.
	client.OpenPopulation(q)
	page, appErr := s.galgameClient.CatalogWorksList(ctx, q)
	if appErr != nil {
		return nil, appErr
	}
	// Items is nil on an empty queue; serialize [] so the page's
	// `data.items.length` reads 0 rather than tripping over null.
	items := page.Items
	if items == nil {
		items = []client.CatalogWorkListItem{}
	}
	return &PendingQueuePage{Items: items, NextCursor: page.NextCursor}, nil
}

// reviewActions is the closed verdict vocabulary this face proxies. `unban` is
// included because a ban is reversible and the queue is where a moderator
// undoes their own; the four owner actions are deliberately absent — a curator
// publishing on a submitter's behalf is a different authority.
var reviewActions = map[string]bool{
	catalogclient.ClaimActionApprove: true,
	catalogclient.ClaimActionDecline: true,
	catalogclient.ClaimActionBan:     true,
	catalogclient.ClaimActionUnban:   true,
}

// Review records one verdict on a submission.
//
// The site is deliberately NOT sent: a review action is cross-tenant authority
// and the registry records the claim's own site on the event. Sending kungal's
// binding here would look like tenancy and behave like nothing.
func (s *ClaimReviewService) Review(
	ctx context.Context,
	actor catalogclient.EditActor,
	gid int,
	action string,
	reason string,
) (*catalogclient.ClaimActionResult, *errors.AppError) {
	if !reviewActions[action] {
		return nil, errors.ErrBadRequest("未知的审核动作")
	}
	if action == catalogclient.ClaimActionDecline && reason == "" {
		// The registry enforces this too; refusing here means the moderator gets
		// the message in their own words rather than a translated 422.
		return nil, errors.ErrValidation("拒绝时必须填写理由")
	}
	ids, appErr := s.galgameClient.CatalogWorkIDs(ctx, []int{gid})
	if appErr != nil {
		return nil, appErr
	}
	workID, ok := ids[gid]
	if !ok {
		return nil, errors.ErrNotFound("条目不存在")
	}
	res, err := s.catalog.ActOnClaim(ctx, workID, action, catalogclient.ClaimActionRequest{
		Actor: actor, Reason: reason,
	})
	if err != nil {
		return nil, claimActionError(err)
	}
	return res, nil
}
