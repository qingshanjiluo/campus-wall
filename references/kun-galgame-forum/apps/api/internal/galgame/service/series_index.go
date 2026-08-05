package service

// The series index, precomputed and cached.
//
// A series card promises "共 N 部 Galgame" and a montage; the page behind it
// lists the LOCAL members that this site can actually open — a member with no
// local row (never ingested) or no download resource is not listed there. Read
// upstream's member count instead and 59 of the 147 groupings with published
// members advertise games and then render an empty page.
//
// So the index is built from what the page will show: each series' members are
// resolved to kungal gids, run through the list repository's own predicate (the
// one the detail page uses), and the survivors are the count, the montage and
// the reason to list the series at all.
//
// That costs one upstream call per series, which is why it is computed once for
// the whole facet and cached rather than per request. A stale index is served
// while the next one builds — a browse list may lag a new resource by minutes;
// it may not hang on 150 upstream calls.

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/model"
	"kun-galgame-api/pkg/errors"
)

const (
	// seriesIndexTTL is how stale a card may be. The inputs move when someone
	// publishes a resource or claims an entry, neither of which is urgent on a
	// browse index.
	seriesIndexTTL = 10 * time.Minute
	// seriesIndexFanout bounds the concurrent member queries during a rebuild.
	// The facet is ~150 series; unbounded would open that many sockets at once
	// against one upstream.
	seriesIndexFanout = 8
	// seriesMemberCap is the member page size for the rebuild. The largest real
	// series is far under it, so one call per series covers the membership.
	seriesMemberCap = 100
)

// indexedSeries is one built card plus the fields the response does not carry.
type indexedSeries struct {
	card    dto.SeriesCard
	hasNSFW *bool
}

// indexRows returns the cached index, rebuilding when it is missing or stale.
func (s *SeriesService) indexRows(ctx context.Context) ([]indexedSeries, *errors.AppError) {
	return s.index.get(ctx, seriesIndexTTL, s.buildIndex)
}

// buildIndex walks the facet and builds every card that has something to show.
func (s *SeriesService) buildIndex(ctx context.Context) ([]indexedSeries, *errors.AppError) {
	lane, appErr := s.walkIndex(ctx)
	if appErr != nil {
		return nil, appErr
	}

	// work_count counts LIVE members, so a zero here cannot survive the local
	// predicate either — skipping those saves ~three quarters of the fan-out.
	candidates := make([]seriesIndexRow, 0, len(lane))
	for _, row := range lane {
		if row.GalgameCount > 0 {
			candidates = append(candidates, row)
		}
	}

	rows := make([]indexedSeries, len(candidates))
	sem := make(chan struct{}, seriesIndexFanout)
	var wg sync.WaitGroup
	for i, row := range candidates {
		wg.Add(1)
		go func(i int, row seriesIndexRow) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			rows[i] = s.buildCard(ctx, row)
		}(i, row)
	}
	wg.Wait()

	out := make([]indexedSeries, 0, len(rows))
	for _, r := range rows {
		// Nothing this site can open — the card would lead to an empty page.
		if r.card.GalgameCount > 0 {
			out = append(out, r)
		}
	}
	// Biggest first: the series a reader recognises are the ones with several
	// entries here, and the lane's own order is the upstream import order, which
	// carries no meaning at all. Name breaks ties so paging is stable.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].card.GalgameCount != out[j].card.GalgameCount {
			return out[i].card.GalgameCount > out[j].card.GalgameCount
		}
		return out[i].card.Name < out[j].card.Name
	})
	return out, nil
}

// buildCard resolves one series' listable members into its card.
func (s *SeriesService) buildCard(ctx context.Context, row seriesIndexRow) indexedSeries {
	card := dto.SeriesCard{
		ID:            row.ID,
		Name:          row.Name,
		IsNSFW:        seriesNSFW(row.hasNSFW, false),
		SampleGalgame: []dto.SeriesSample{},
	}

	members, appErr := s.galgameClient.CatalogWorksSearch(ctx, client.OpenPopulation(url.Values{
		"series_id":   {strconv.Itoa(row.ID)},
		"page":        {"1"},
		"limit":       {strconv.Itoa(seriesMemberCap)},
		"include":     {CatalogCardInclude},
		"claim_state": {"live"},
		// Oldest first: the montage reads as 1作 → 続編, and the title line
		// ("《x》、《y》 等") then names the entries a reader recognises.
		"sort": {"released_asc"},
	}))
	if appErr != nil {
		return indexedSeries{card: card, hasNSFW: row.hasNSFW}
	}

	// The projection carries the kungal gid (claimed_by.work_id), which is the
	// id the local tables are keyed by; a member no product claims has none and
	// drops out here.
	items := make([]dto.NextMoeGalgameItem, 0, len(members.Items))
	gids := make([]int, 0, len(members.Items))
	for i := range members.Items {
		if !client.CatalogItemRenderable(&members.Items[i]) {
			continue
		}
		it := client.CatalogItemToNextMoeItem(&members.Items[i])
		if it.ID <= 0 {
			continue
		}
		items = append(items, it)
		gids = append(gids, it.ID)
	}

	listable := s.listableGIDs(gids)
	sampleNSFW := false
	for _, it := range items {
		if !listable[it.ID] {
			continue
		}
		card.GalgameCount++
		if len(card.SampleGalgame) >= seriesSampleSize {
			continue
		}
		if it.ContentLimit == "nsfw" {
			sampleNSFW = true
		}
		card.SampleGalgame = append(card.SampleGalgame, dto.SeriesSample{
			Name: dto.KunLanguage{
				EnUs: it.NameEnUs, JaJp: it.NameJaJp,
				ZhCn: it.NameZhCn, ZhTw: it.NameZhTw,
			},
			EffectiveBannerHash:      it.EffectiveBannerHash,
			EffectiveBannerURL:       it.EffectiveBannerURL,
			EffectiveBannerThumbhash: it.EffectiveBannerThumbhash,
		})
	}
	card.IsNSFW = seriesNSFW(row.hasNSFW, sampleNSFW)
	return indexedSeries{card: card, hasNSFW: row.hasNSFW}
}

// listableGIDs narrows member gids to the ones the series PAGE will list, using
// that page's own query: a local row, and — unless the reader turned the
// setting on, which the entity pages never do — a download resource.
//
// Order is not preserved; the caller walks its own gid list against the set.
func (s *SeriesService) listableGIDs(gids []int) map[int]bool {
	out := make(map[int]bool, len(gids))
	if len(gids) == 0 {
		return out
	}
	if s.galgameSvc == nil {
		// No local repository wired (the taxonomy-only tests): nothing to
		// subtract, so every member counts.
		for _, gid := range gids {
			out[gid] = true
		}
		return out
	}
	ids, _ := s.galgameSvc.listRepo.ListIDs(model.GalgameListFilter{
		RestrictIDs: gids,
		Page:        1,
		Limit:       len(gids),
		SortOrder:   "desc",
	})
	for _, id := range ids {
		out[id] = true
	}
	return out
}
