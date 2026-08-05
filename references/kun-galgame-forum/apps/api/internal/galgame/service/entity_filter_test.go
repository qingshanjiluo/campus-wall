package service

import (
	"net/url"
	"testing"

	"kun-galgame-api/internal/galgame/dto"
)

// buildEntityFilter must read the FE's camelCase browse keys straight through
// (the entity pages filter the forum-LOCAL subset with the same vocabulary as
// /galgame), and scope the whole list to the entity's members via RestrictIDs.
func TestBuildEntityFilter(t *testing.T) {
	q := url.Values{}
	q.Set("type", "patch")
	q.Set("language", "zh-cn")
	q.Set("platform", "windows")
	q.Set("gameType", "moe")
	q.Set("sortField", "view_7d")
	q.Set("sortOrder", "asc")
	q.Set("page", "3")
	q.Set("limit", "24")
	q.Set("showNoResource", "true")

	f := buildEntityFilter(q, []int{5, 9, 12})

	if f.Type != "patch" || f.Language != "zh-cn" || f.Platform != "windows" {
		t.Errorf("resource filters not read: %+v", f)
	}
	if f.GameType != "moe" {
		t.Errorf("GameType = %q, want moe (chip must not be dead)", f.GameType)
	}
	if f.SortField != "view_7d" || f.SortOrder != "asc" {
		t.Errorf("sort = %q/%q, want view_7d/asc", f.SortField, f.SortOrder)
	}
	if f.Page != 3 || f.Limit != 24 {
		t.Errorf("page/limit = %d/%d, want 3/24", f.Page, f.Limit)
	}
	if !f.ShowNoResource {
		t.Error("ShowNoResource should be true")
	}
	if len(f.RestrictIDs) != 3 || f.RestrictIDs[0] != 5 {
		t.Errorf("RestrictIDs = %v, want [5 9 12]", f.RestrictIDs)
	}
}

func TestBuildEntityFilterDefaults(t *testing.T) {
	f := buildEntityFilter(url.Values{}, []int{})

	if f.SortOrder != "desc" {
		t.Errorf("SortOrder default = %q, want desc", f.SortOrder)
	}
	if f.Page != 1 || f.Limit != 24 {
		t.Errorf("page/limit default = %d/%d, want 1/24", f.Page, f.Limit)
	}
	if f.ShowNoResource {
		t.Error("ShowNoResource default should be false")
	}
	// A non-nil but EMPTY RestrictIDs must stay non-nil: list_repo reads nil as
	// "no restriction" (whole catalogue) and non-nil as "restrict to this set".
	// An entity with zero members must render empty, never the full catalogue —
	// EntityGalgameIDs guarantees non-nil, and buildEntityFilter must preserve it.
	if f.RestrictIDs == nil {
		t.Error("RestrictIDs must stay non-nil so an empty entity restricts to nothing")
	}
}

// Local-subset cards are always on-forum (a game with no local row can't survive
// the list_repo filter), so no card renders as a galgame-only "未收录" entry.
func TestListCardsToEntityCardsIsOnForum(t *testing.T) {
	out := listCardsToEntityCards([]dto.GalgameListCard{{ID: 1, View: 10}})
	if len(out) != 1 || out[0].ID != 1 || out[0].View != 10 {
		t.Fatalf("field copy wrong: %+v", out)
	}
	if !out[0].IsOnForum {
		t.Error("IsOnForum should be true for local-subset cards")
	}
}
