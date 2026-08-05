package service

import (
	"context"
	"net/url"
	"strconv"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// EngineService serves the engine index + detail lanes off the catalog engine
// facet.
//
// VNDB publishes no engine data, so this facet's only copy anywhere is the
// hand-curated wiki one the retirement wave migrated — a few hundred rows,
// which is why the catalog ships description + aliases inline on the LIST row
// and this service can render the whole index without a per-row round-trip.
type EngineService struct {
	galgameClient *client.GalgameClient
	// galgameSvc runs the shared local filter/sort/paginate + hydration flow
	// over the engine's member ids (the catalog can't filter by kungal-local
	// resource data). See GetDetail.
	galgameSvc *GalgameService
}

func NewEngineService(galgameClient *client.GalgameClient, galgameSvc *GalgameService) *EngineService {
	return &EngineService{galgameClient: galgameClient, galgameSvc: galgameSvc}
}

// engineIndexPageCap bounds the index walk. The facet is ~200 rows, so two
// upstream pages cover it; the cap is a backstop, not a working limit.
const engineIndexPageCap = 20

// GetList — GET /galgame-engine
//
// The FE engine index renders every engine at once (no pager), so the keyset
// lane is walked to exhaustion here.
func (s *EngineService) GetList(ctx context.Context) ([]dto.EngineListItem, *errors.AppError) {
	items := []dto.EngineListItem{}
	cursor := ""
	for page := 0; page < engineIndexPageCap; page++ {
		q := client.OpenPopulation(url.Values{"limit": {"100"}})
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		res, appErr := s.galgameClient.CatalogTaxonomyList(ctx, "engines", q)
		if appErr != nil {
			return nil, appErr
		}
		for _, e := range res.Items {
			items = append(items, dto.EngineListItem{
				ID:           int(e.ID),
				Name:         e.Label(),
				Description:  e.Description,
				Alias:        emptyStrSliceIfNil(e.Aliases),
				GalgameCount: e.WorkCount,
			})
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}
	return items, nil
}

// GetDetail — GET /galgame-engine/:id (id = a catalog ENGINE id)
func (s *EngineService) GetDetail(
	ctx context.Context,
	id string,
	rawQuery url.Values,
	isSFW bool,
) (*dto.EngineDetail, *errors.AppError) {
	e, found, appErr := s.galgameClient.CatalogEngine(ctx, id)
	if appErr != nil {
		return nil, appErr
	}
	if !found {
		return nil, errors.ErrNotFound("未找到该引擎")
	}

	// Entity detail lists the forum-LOCAL subset of the engine's catalogue, so
	// the kungal filters (类型/语言/平台/作品类型) + every sort work.
	memberIDs, appErr := s.galgameClient.CatalogMemberGIDs(ctx,
		url.Values{"engine_id": {id}}, isSFW, taxonomyMemberPageCap)
	if appErr != nil {
		return nil, appErr
	}
	page, appErr := s.galgameSvc.hydrateListCards(ctx, buildEntityFilter(rawQuery, memberIDs), isSFW)
	if appErr != nil {
		return nil, appErr
	}

	return &dto.EngineDetail{
		ID:          int(e.ID),
		Name:        e.Name,
		Description: e.Description,
		Alias:       emptyStrSliceIfNil(e.Aliases),
		Galgame:     listCardsToEntityCards(page.Galgames),
		// Same gated page as the rows — never e.WorkCount (upstream counts the
		// engine's whole catalogue, published or not, forum-local or not).
		GalgameCount: page.Total,
	}, nil
}

func emptyStrSliceIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// atoiOrZero parses an id path segment, returning 0 when it is not a positive
// integer — the caller then 404s rather than forwarding garbage upstream.
func atoiOrZero(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
