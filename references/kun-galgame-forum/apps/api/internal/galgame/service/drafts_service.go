package service

import (
	"context"
	"net/url"
	"strconv"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// DraftsService serves the "未发布的游戏" claim funnel: works the catalog knows
// about that kungal has never ingested, optionally scoped to one taxonomy
// entity (the modal lives on the entity detail pages).
//
// The wiki used to answer this with its own unclaimed VNDB drafts (status=2).
// The catalog answers it better and in the CATALOG id space: `claimed=false` is
// literally "no product has an entry for this work", which is exactly what the
// funnel is offering the user the chance to change. Every card therefore comes
// back IsOnForum=false with the claim-card status, and the card's link is
// name-based (the publish wizard pre-searched by title) — no kungal id needed,
// which is what makes an unclaimed work renderable at all.
type DraftsService struct {
	galgameClient *client.GalgameClient
	enricher      *GalgameEnricher
}

func NewDraftsService(galgameClient *client.GalgameClient, enricher *GalgameEnricher) *DraftsService {
	return &DraftsService{galgameClient: galgameClient, enricher: enricher}
}

// DraftFilters optionally scopes the list to one taxonomy entity. Ids are
// CATALOG ids (the entity pages carry those now); a zero id means "no filter on
// that dimension", so an all-zero value reproduces the global list.
type DraftFilters struct {
	LabelID  int
	TagID    int
	EngineID int
	SeriesID int
	// OriginalLanguages is a CSV of catalog original-language tags; empty =
	// the face's own default (ja + the zh family).
	OriginalLanguages string
}

// GetDrafts returns one page of unclaimed works as enriched claim cards.
// page/limit are pre-clamped by the handler.
func (s *DraftsService) GetDrafts(
	ctx context.Context,
	page, limit int,
	f DraftFilters,
) (*dto.DraftsPage, *errors.AppError) {
	q := url.Values{
		"claimed": {"false"},
		"page":    {strconv.Itoa(page)},
		"limit":   {strconv.Itoa(limit)},
		"include": {CatalogCardInclude},
		// Newest announcements first: the funnel is about what is missing NOW,
		// not about the back catalogue.
		"sort": {"released_desc"},
	}
	if f.LabelID > 0 {
		q.Set("label_id", strconv.Itoa(f.LabelID))
	}
	if f.TagID > 0 {
		q.Set("tag_id", strconv.Itoa(f.TagID))
	}
	if f.EngineID > 0 {
		q.Set("engine_id", strconv.Itoa(f.EngineID))
	}
	if f.SeriesID > 0 {
		q.Set("series_id", strconv.Itoa(f.SeriesID))
	}
	if f.OriginalLanguages != "" {
		q.Set("olang", f.OriginalLanguages)
	}
	// The age gate is open like everywhere else. The EDITORIAL gate is
	// deliberately absent here and only here: this lane pins claimed=false, so
	// by contract every row has claimed_by null and therefore no editorial
	// verdict to filter on. Sending content_limit=sfw could only ever be a
	// no-op or — if the face reads a missing verdict as a non-match — empty the
	// whole claim funnel for the SFW default. Nothing is leaked either way: a
	// work no product has claimed has no product-sanitized presentation to hide.
	client.OpenPopulation(q)

	res, appErr := s.galgameClient.CatalogWorksSearch(ctx, q)
	if appErr != nil {
		return nil, appErr
	}
	return &dto.DraftsPage{
		Items: s.enricher.ToCards(ctx, catalogItemsToNextMoe(res.Items)),
		Total: res.Total,
	}, nil
}
