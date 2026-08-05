package service

import (
	"context"
	"strconv"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/userclient"
)

// GalgameProxyService handles pass-through proxying to the galgame service and the
// common "galgame + local user resolution" pattern used by galgame sub-routes.
type GalgameProxyService struct {
	galgameClient *client.GalgameClient
	galgameRepo   *repository.GalgameRepository
	userClient    *userclient.Client
}

func NewGalgameProxyService(
	galgameClient *client.GalgameClient,
	galgameRepo *repository.GalgameRepository,
	userClient *userclient.Client,
) *GalgameProxyService {
	return &GalgameProxyService{galgameClient: galgameClient, galgameRepo: galgameRepo, userClient: userClient}
}

// ──────────────────────────────────────────
// Galgame Links
// ──────────────────────────────────────────

type nextMoeLinkRow struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Link      string `json:"link"`
	GalgameID int    `json:"galgame_id"`
	UserID    int    `json:"user_id"`
}

// GetGalgameLinks fetches a galgame's curated external links.
//
// These are platform-curated rows now: the retirement wave absorbed the wiki's
// user-submitted links WITHOUT their submitter, so there is no author to
// resolve and the banned-author filter has no subject any more (doc 126 D6 —
// the user_id field sunsets with the wiki). Link moderation for new rows runs
// through the editing engine's own review chain instead.
func (s *GalgameProxyService) GetGalgameLinks(
	ctx context.Context,
	gid string,
) ([]dto.GalgameLink, *errors.AppError) {
	gidInt, err := strconv.Atoi(gid)
	if err != nil || gidInt <= 0 {
		return nil, errors.ErrBadRequest("无效的 Galgame ID")
	}
	rows, appErr := s.galgameClient.CatalogWorkLinks(ctx, gidInt)
	if appErr != nil {
		return nil, appErr
	}
	out := make([]dto.GalgameLink, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.GalgameLink{
			GalgameID: gidInt,
			Name:      r.Name,
			Link:      r.Link,
		})
	}
	return out, nil
}

// The old-wire galgame revision/PR list reads (GetGalgameHistory /
// GetGalgamePRs and their row shapes) retired in E3b — kungal reads the
// editing engine now (edit_handler.go).
