package handler

import (
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/rss/dto"
	"kun-galgame-api/internal/rss/repository"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/userclient"

	"github.com/gofiber/fiber/v3"
)

// RSSHandler handles RSS feed routes.
// No service layer — logic is a single query with fixed filters.
// Galgame RSS additionally enriches local stub IDs with galgame metadata.
type RSSHandler struct {
	repo          *repository.RSSRepository
	galgameClient *client.GalgameClient
	userClient    *userclient.Client
}

func NewRSSHandler(
	repo *repository.RSSRepository,
	galgameClient *client.GalgameClient,
	userClient *userclient.Client,
) *RSSHandler {
	return &RSSHandler{repo: repo, galgameClient: galgameClient, userClient: userClient}
}

// GetTopicRSS returns recent topics for RSS feed.
// GET /api/rss/topic
func (h *RSSHandler) GetTopicRSS(c fiber.Ctx) error {
	rows := h.repo.FindRecentSFWTopics()
	uids := userclient.CollectIDs(rows, func(r dto.TopicRSSItem) int { return r.UserID })
	userMap := h.userClient.Hydrate(c.Context(), uids)
	items := make([]dto.TopicRSSItem, 0, len(rows))
	for i := range rows {
		u := userMap[rows[i].UserID]
		// Drop topics authored by a banned user from the syndicated feed.
		if !userclient.IsRenderable(u) {
			continue
		}
		rows[i].UserName = u.Name
		items = append(items, rows[i])
	}
	return response.OK(c, items)
}

// GetGalgameRSS returns the 10 most recent galgames as RSS items.
// GET /api/rss/galgame
//
// Local DB stores the stub ID, the created timestamp and the frozen creator —
// name/banner come from the galgame batch endpoint. Description is left empty
// since galgame batch doesn't include intros.
func (h *RSSHandler) GetGalgameRSS(c fiber.Ctx) error {
	rows := h.repo.FindRecentGalgameIDs(10)
	if len(rows) == 0 {
		return response.OK(c, []dto.GalgameRSSItem{})
	}

	ids := make([]int, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	// RSS is consumed by feed readers and search engines — pin SFW
	// unconditionally (docs/galgame_wiki/00-handbook §16). Anything
	// else would leak NSFW into syndicated channels we don't control.
	briefMap, _ := h.galgameClient.GetBatchPublic(c.Context(), ids, true)
	if briefMap == nil {
		briefMap = map[int]client.GalgameBrief{}
	}

	// The feed's author is the FROZEN wiki-era creator on the LOCAL row
	// (migration 066): the catalog face carries no submitter, so a brief's own
	// user_id is always 0 — hydrating off it asked OAuth about user 0 and
	// syndicated every galgame as authored by 已注销用户.
	userMap := h.userClient.Hydrate(c.Context(), userclient.CollectIDs(rows,
		func(r repository.RecentGalgameRow) int { return userclient.DerefID(r.CreatorUserID) }))

	items := make([]dto.GalgameRSSItem, 0, len(rows))
	for _, row := range rows {
		b, ok := briefMap[row.ID]
		if !ok {
			continue
		}
		u := userMap[userclient.DerefID(row.CreatorUserID)]
		items = append(items, dto.GalgameRSSItem{
			ID:     row.ID,
			Name:   pickPreferredName(b),
			Banner: b.Banner,
			User: dto.GalgameRSSUser{
				ID: u.ID, Name: u.Name, Avatar: u.Avatar,
			},
			Description: "",
			Created:     row.Created,
		})
	}
	return response.OK(c, items)
}

// pickPreferredName mirrors the FE getPreferredLanguageText zh-cn default
// fallback chain: zh-cn > zh-tw > ja-jp > en-us. Returns the first non-empty
// entry. en-US (usually the VNDB romaji title) is LAST so a JP/CN-titled game
// never surfaces its VNDB English name when a Chinese/Japanese name exists.
func pickPreferredName(b client.GalgameBrief) string {
	candidates := []string{b.NameZhCn, b.NameZhTw, b.NameJaJp, b.NameEnUs}
	for _, n := range candidates {
		if n != "" {
			return n
		}
	}
	return ""
}
