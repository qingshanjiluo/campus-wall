package service

import (
	"context"
	"net/url"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// OfficialService serves the 会社 browse / search / detail lanes.
//
// The catalog calls this entity a LABEL; the kungal product wording (会社) is
// unchanged, so the rename shows up only in the upstream path and the new URL
// space. Two field-level notes:
//
//   - `category` is now the label KIND (game_brand / publisher / doujin_circle
//     …), a richer and better-defined vocabulary than the wiki's free-text one.
//   - `original` (the wiki's original-language name field) has no catalog
//     counterpart on the public face. It survives only where it is still
//     WRITTEN — the staff edit form, which reads it back off the /api face.
type OfficialService struct {
	galgameClient *client.GalgameClient
	// galgameSvc runs the shared local filter/sort/paginate + hydration flow
	// over the label's member ids (the catalog can't filter by kungal-local
	// resource data). See GetDetail.
	galgameSvc *GalgameService
}

func NewOfficialService(galgameClient *client.GalgameClient, galgameSvc *GalgameService) *OfficialService {
	return &OfficialService{galgameClient: galgameClient, galgameSvc: galgameSvc}
}

// GetList — GET /galgame-official
//
// has_works=1 drops the empty vocabulary. The label词表 is 37,623 rows and some
// 40% of them have no works — imported organisations nothing here credits — so
// browsing it unfiltered was mostly "+ 0" cards. The filter is the same
// predicate upstream counts with (so a listed 会社 always has something to
// show) and `total` converges with it, which is why this is a query parameter
// rather than a filter applied to the page after the fact: dropping rows
// locally would leave short pages under an inflated pager.
func (s *OfficialService) GetList(
	ctx context.Context,
	rawQuery url.Values,
) (*dto.OfficialListPage, *errors.AppError) {
	base := client.OpenPopulation(url.Values{"has_works": {"1"}})
	if kind := rawQuery.Get("kind"); kind != "" {
		base.Set("kind", kind)
	}
	rows, total, appErr := s.galgameClient.CatalogTaxonomyPageAt(ctx, "labels", base,
		atoiOr(rawQuery.Get("page"), 1), atoiOr(rawQuery.Get("limit"), 50))
	if appErr != nil {
		return nil, appErr
	}

	items := make([]dto.OfficialListItem, 0, len(rows))
	for _, o := range rows {
		items = append(items, dto.OfficialListItem{
			ID:   int(o.ID),
			Name: o.Label(),
			// The browse row is identity + count; the maker's website lives on
			// the record (links[]), which the detail page fetches. The list
			// cards never rendered a website anyway.
			Category:     o.Kind,
			Alias:        emptyStrSliceIfNil(o.Aliases),
			GalgameCount: o.WorkCount,
		})
	}
	return &dto.OfficialListPage{Officials: items, Total: total}, nil
}

// Search — GET /galgame-official/search
//
// Identity only: the upstream search face returns id + name and nothing else,
// so this answers in the search shape rather than dressing a hit up as a browse
// row whose count, category and language would all be zero values.
//
// The frontend does `searchResult.value = res` expecting a bare array, so the
// envelope is unwrapped here and the gateway response shape stays put.
func (s *OfficialService) Search(
	ctx context.Context,
	rawQuery url.Values,
) ([]dto.TaxonomySearchItem, *errors.AppError) {
	hits, appErr := s.galgameClient.CatalogEntitySearch(ctx, "labels",
		rawQuery.Get("q"), atoiOr(rawQuery.Get("limit"), 20))
	if appErr != nil {
		return nil, appErr
	}
	items := make([]dto.TaxonomySearchItem, 0, len(hits))
	for _, o := range hits {
		items = append(items, dto.TaxonomySearchItem{ID: int(o.ID), Name: o.Name})
	}
	return items, nil
}

// GetDetail — GET /galgame-official/:id (id = a catalog LABEL id)
//
// Entity detail lists the forum-LOCAL subset of the label's catalogue, so the
// kungal filters (类型/语言/平台/作品类型) + every sort work. Only the label's
// metadata comes from upstream; the galgame list is recomputed locally from the
// member ids below.
func (s *OfficialService) GetDetail(
	ctx context.Context,
	id string,
	rawQuery url.Values,
	isSFW bool,
) (*dto.OfficialDetail, *errors.AppError) {
	o, found, movedTo, appErr := s.galgameClient.CatalogLabel(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	// Merged away upstream: hand the page the survivor's id and nothing else,
	// so it can 301 in one hop instead of rendering a ghost — a dead label used
	// to answer with its old name and an empty catalogue, which reads as a real
	// but empty company page.
	if movedTo != 0 {
		return &dto.OfficialDetail{MovedTo: int(movedTo)}, nil
	}
	if !found {
		return nil, errors.ErrNotFound("未找到该会社")
	}

	memberIDs, appErr := s.galgameClient.CatalogMemberGIDs(ctx,
		url.Values{"label_id": {id}}, isSFW, taxonomyMemberPageCap)
	if appErr != nil {
		return nil, appErr
	}
	page, appErr := s.galgameSvc.hydrateListCards(ctx, buildEntityFilter(rawQuery, memberIDs), isSFW)
	if appErr != nil {
		return nil, appErr
	}

	return &dto.OfficialDetail{
		ID:   int(o.ID),
		Name: o.DisplayName,
		// The wiki's separate `original` field has no public catalog
		// counterpart; the label's own name IS the original in the common case
		// (a Japanese brand is stored under its Japanese name), and the
		// alternate spellings are in alias[].
		Links:       officialLinks(o.Links),
		Link:        client.PrimaryLabelLink(o),
		Category:    o.Kind,
		Lang:        o.Lang,
		Description: preferredIntro(o.Intros),
		Alias:       emptyStrSliceIfNil(o.Aliases),
		Galgame:     listCardsToEntityCards(page.Galgames),
		// The header count and the pager MUST come from the same gated page the
		// rows do. o.WorkCount (upstream, nsfw-aware) is right there and is the
		// wrong answer: it counts the label's whole catalogue, published or not
		// and forum-local or not, so using it would put a pager on works this
		// page cannot list.
		GalgameCount: page.Total,
	}, nil
}

// officialLinks passes the label's web presences through with their source
// keys, so the page can name each one instead of labelling an X account
// "官方网站". Always a slice, never null — the FE iterates it unguarded.
func officialLinks(links []client.CatalogLabelLink) []dto.OfficialLink {
	out := make([]dto.OfficialLink, 0, len(links))
	for _, l := range links {
		out = append(out, dto.OfficialLink{Source: l.Source, URL: l.URL})
	}
	return out
}

// ResolveLegacyID maps a legacy wiki 会社 id onto its catalog label id.
func (s *OfficialService) ResolveLegacyID(ctx context.Context, wikiID int) (int64, bool, *errors.AppError) {
	return s.galgameClient.LookupWikiLabel(ctx, wikiID)
}
