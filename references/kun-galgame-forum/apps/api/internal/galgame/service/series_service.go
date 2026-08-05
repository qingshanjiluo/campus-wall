package service

import (
	"context"
	"net/url"
	"strconv"
	"sync"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// SeriesService serves the series detail lane off the catalog series facet
// (catalog_series + catalog_series_member).
//
// A series is the one grouping entity a reader arrives at from a game rather
// than from an index — "这游戏属于哪个系列, 还有哪几部" — so the page it lands on
// is the same shape as the other three entity pages: the forum-LOCAL subset of
// the series' member works, filterable and sortable like /galgame itself.
type SeriesService struct {
	galgameClient *client.GalgameClient
	// galgameSvc runs the shared local filter/sort/paginate + hydration flow
	// over the series' member ids (the catalog cannot filter by kungal-local
	// resource data). Same arrangement as EngineService.
	galgameSvc *GalgameService
	// index caches the built browse cards — see series_index.go.
	index staleCache[indexedSeries]
}

func NewSeriesService(galgameClient *client.GalgameClient, galgameSvc *GalgameService) *SeriesService {
	return &SeriesService{galgameClient: galgameClient, galgameSvc: galgameSvc}
}

// seriesIndexPageCap bounds the index walk. ~600 rows today, so six upstream
// pages cover it; the cap is a backstop, not a working limit.
const seriesIndexPageCap = 20

// GetList — GET /galgame-series
//
// Walks the catalog series browse lane to exhaustion, like the engine index:
// the facet has no search of its own (no Meilisearch index — see the lane's own
// doc), so the consumer that needs to find a series BY NAME filters the full
// set client-side. That is affordable at this size and stops being so long
// before the cap.
func (s *SeriesService) GetList(ctx context.Context) ([]dto.SeriesListItem, *errors.AppError) {
	rows, appErr := s.walkIndex(ctx)
	if appErr != nil {
		return nil, appErr
	}
	items := make([]dto.SeriesListItem, len(rows))
	for i, r := range rows {
		items[i] = r.SeriesListItem
	}
	return items, nil
}

// seriesIndexRow is one walked index row plus the content answer the DTO does
// not carry: whether the series holds any adult member.
type seriesIndexRow struct {
	dto.SeriesListItem
	hasNSFW *bool
}

// walkIndex reads the whole series lane.
//
// Always the open population: the age gate stays open on every identity lane
// here, and what an SFW reader may see is decided by each row's has_nsfw, not
// by asking for a smaller catalogue.
func (s *SeriesService) walkIndex(ctx context.Context) ([]seriesIndexRow, *errors.AppError) {
	items := []seriesIndexRow{}
	cursor := ""
	for page := 0; page < seriesIndexPageCap; page++ {
		q := client.OpenPopulation(url.Values{"limit": {"100"}})
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		res, appErr := s.galgameClient.CatalogTaxonomyList(ctx, "series", q)
		if appErr != nil {
			return nil, appErr
		}
		for _, e := range res.Items {
			items = append(items, seriesIndexRow{
				SeriesListItem: dto.SeriesListItem{
					ID:           int(e.ID),
					Name:         e.Label(),
					GalgameCount: e.WorkCount,
				},
				hasNSFW: e.HasNSFW,
			})
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	return items, nil
}

// seriesSampleSize is how many member works a series CARD fans out behind its
// title. Five is the montage's own limit — the sixth cover would be hidden
// under the fifth.
const seriesSampleSize = 5

// GetCards — GET /galgame-series/cards
//
// The rich index card: identity + member count + a five-work cover montage.
// Two callers, one shape — the series index (paged) and the series panel on a
// game's detail page (`ids`), which is why the id list is a filter here rather
// than a second endpoint answering a second shape.
//
// The samples cost one upstream works query per series, run together. That is
// affordable for one page of twelve and for the one or two series a game
// belongs to, and it is why this is NOT what the bare /galgame-series lane
// returns: the editor's picker walks the whole ~600-row facet and must stay a
// handful of calls.
func (s *SeriesService) GetCards(
	ctx context.Context,
	ids []int,
	page, limit int,
	isSFW bool,
) (*dto.SeriesCardPage, *errors.AppError) {
	// A game's panel names its series, so it needs neither the index nor its
	// order: each card is built from the series record and its own member
	// query, two calls that run per id instead of a whole-facet walk on every
	// galgame detail view.
	if len(ids) > 0 {
		return s.cardsByID(ctx, ids, isSFW), nil
	}

	// Every card is precomputed (see series_index.go): the cards are already
	// down to the series this site can show, biggest first.
	index, appErr := s.indexRows(ctx)
	if appErr != nil {
		return nil, appErr
	}

	// An SFW reader sees only the series with no adult member at all. A series
	// is a grouping, so the per-work gate the rest of the catalogue uses would
	// leave a fragment — half a series, with the montage and the count
	// disagreeing about which half.
	rows := make([]dto.SeriesCard, 0, len(index))
	for _, it := range index {
		if isSFW && !sfwSafeSeries(it.hasNSFW) {
			continue
		}
		rows = append(rows, it.card)
	}
	total := int64(len(rows))

	// Filtering happens BEFORE the slice, so the pager never promises pages
	// that turn out empty.
	if start := (page - 1) * limit; start >= len(rows) {
		rows = nil
	} else {
		rows = rows[start:min(start+limit, len(rows))]
	}
	return &dto.SeriesCardPage{Series: rows, Total: total}, nil
}

// sfwSafeSeries answers "may an SFW reader see this series".
//
// The axis is has_nsfw — the catalog's aggregate of its members' EDITORIAL
// content_limit, the same axis every other content gate here reads. Not the
// age rating: an r18-rated work whose claim says sfw is shown by this product
// everywhere else, and a series of such works is a normal SFW series.
//
// nil = the catalog did not answer, which is not the same as "clean": a
// deployment that predates the field must not put adult series in front of a
// reader who asked not to see them. Unanswered therefore reads as unsafe.
func sfwSafeSeries(hasNSFW *bool) bool {
	return hasNSFW != nil && !*hasNSFW
}

// seriesNSFW is what the card's chip says. The catalog's own aggregate when it
// gives one; otherwise the sample's — which reads only the five works behind
// the montage, and so can call an adult series clean when its adult entries sit
// past the fifth.
func seriesNSFW(hasNSFW *bool, sampled bool) bool {
	if hasNSFW != nil {
		return *hasNSFW
	}
	return sampled
}

// cardsByID builds the cards a game's detail panel asks for, in the order it
// asked. Built fresh rather than read off the index: a game names one or two
// series, and the record + one member query is cheaper than waiting on a facet
// rebuild — and it also answers for a series the index skipped.
//
// Best-effort per id: a series that fails to resolve is left out rather than
// failing the panel, and an unknown id simply has no card.
func (s *SeriesService) cardsByID(ctx context.Context, ids []int, isSFW bool) *dto.SeriesCardPage {
	cards := make([]*dto.SeriesCard, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i, id int) {
			defer wg.Done()
			rec, found, appErr := s.galgameClient.CatalogSeries(ctx, strconv.Itoa(id))
			if appErr != nil || !found {
				return
			}
			// Same rule as the index, off the record's own flag.
			if isSFW && !sfwSafeSeries(rec.HasNSFW) {
				return
			}
			built := s.buildCard(ctx, seriesIndexRow{
				SeriesListItem: dto.SeriesListItem{ID: int(rec.ID), Name: rec.DisplayName},
				hasNSFW:        rec.HasNSFW,
			})
			// Nothing this site can list: the panel links to a page that would
			// come up empty, so it shows no card at all.
			if built.card.GalgameCount == 0 {
				return
			}
			cards[i] = &built.card
		}(i, id)
	}
	wg.Wait()

	out := make([]dto.SeriesCard, 0, len(cards))
	for _, c := range cards {
		if c != nil {
			out = append(out, *c)
		}
	}
	return &dto.SeriesCardPage{Series: out, Total: int64(len(out))}
}

// GetDetail — GET /galgame-series/:id (id = a catalog SERIES id)
func (s *SeriesService) GetDetail(
	ctx context.Context,
	id string,
	rawQuery url.Values,
	isSFW bool,
) (*dto.SeriesDetail, *errors.AppError) {
	rec, found, appErr := s.galgameClient.CatalogSeries(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if !found {
		return nil, errors.ErrNotFound("未找到该系列")
	}

	memberIDs, appErr := s.galgameClient.CatalogMemberGIDs(ctx,
		url.Values{"series_id": {id}}, isSFW, taxonomyMemberPageCap)
	if appErr != nil {
		return nil, appErr
	}
	page, appErr := s.galgameSvc.hydrateListCards(ctx, buildEntityFilter(rawQuery, memberIDs), isSFW)
	if appErr != nil {
		return nil, appErr
	}

	return &dto.SeriesDetail{
		ID:          int(rec.ID),
		Name:        rec.DisplayName,
		Description: seriesIntro(rec),
		Galgame:     listCardsToEntityCards(page.Galgames),
		// The gated page's own total, never the upstream member count: that one
		// counts the series' whole catalogue, published here or not.
		GalgameCount: page.Total,
	}, nil
}

// seriesIntro picks the blurb to render under the title.
//
// The catalog does NOT merge series intros to one row per language — a
// hand-written rescue and a source's own text both survive — so this takes the
// first row of the reader's preferred language rather than concatenating them:
// two descriptions of the same series stacked on one page reads as a bug.
// Chinese first, then Japanese, then whatever exists; empty is fine, the
// header renders without a description.
func seriesIntro(rec *client.CatalogSeriesDetail) string {
	for _, lang := range []string{"zh-Hans", "zh-Hant", "ja", "en"} {
		for _, in := range rec.Intros {
			if in.Lang == lang && in.Intro != "" {
				return in.Intro
			}
		}
	}
	for _, in := range rec.Intros {
		if in.Intro != "" {
			return in.Intro
		}
	}
	return ""
}
