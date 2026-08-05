package service

// The 发布人 chip on every galgame card must be keyed by the FROZEN wiki-era
// creator on the local row (migration 066), never by the catalog brief's /
// item's own UserID — the catalog face carries no submitter by design, so that
// field is ALWAYS 0 and the chip came back blank site-wide.
//
// Both card lanes route their user lookup through the two helpers pinned here:
//   - GalgameService.HydrateCardsByIDs (/galgame list, entity pages, 收藏夹)
//   - GalgameEnricher.ToCards          (search / series / official / engine / tag)
//
// They are pinned at helper level because the lanes themselves need a live
// Postgres + OAuth to reach (this package has no DB harness) — which is exactly
// how the bug survived review: `brief.UserID` and `localRow.CreatorUserID` are
// both "an int-ish user id" to the compiler, so swapping them type-checks.

import (
	"testing"

	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/internal/galgame/repository"
	"kun-galgame-api/pkg/userclient"
)

func creatorID(id int) *int { return &id }

// userMapWithCatalogDecoy is the map the lanes actually hold: the real creator
// under his own id, plus a decoy under id 0 standing in for what a Hydrate over
// the catalog face's always-zero UserID would have produced. Any lane that
// keys off the brief lands on the decoy.
func userMapWithCatalogDecoy() map[int]userclient.User {
	return map[int]userclient.User{
		0:  userclient.Placeholder(0),
		42: {ID: 42, Name: "鲲", Avatar: "https://example.invalid/kun.webp"},
	}
}

func TestFrozenCreatorBrief_ReadsLocalSnapshotNotCatalogFace(t *testing.T) {
	got := frozenCreatorBrief(
		repository.GalgameLocalRow{ID: 7, CreatorUserID: creatorID(42)},
		userMapWithCatalogDecoy(),
	)

	if got.ID != 42 || got.Name != "鲲" {
		t.Fatalf("brief = %+v, want the frozen local creator (id 42) — a zero/placeholder "+
			"here means the lane is keying off the catalog brief's UserID again", got)
	}
	if got.Avatar == "" {
		t.Errorf("avatar = %q, want the creator's — the chip renders name + avatar", got.Avatar)
	}
}

func TestFrozenCreatorBrief_UnknownCreatorRendersNoChip(t *testing.T) {
	rows := map[string]repository.GalgameLocalRow{
		// creator_user_id IS NULL: a row the backfill never reached.
		"null creator": {ID: 7},
		// localMap[id] miss: a catalog-only work the forum never ingested,
		// which hands the zero value straight to the helper.
		"no local row": {},
	}
	for name, row := range rows {
		t.Run(name, func(t *testing.T) {
			got := frozenCreatorBrief(row, userMapWithCatalogDecoy())
			if got != (dto.UserBrief{}) {
				t.Fatalf("brief = %+v, want the zero brief — an unknown creator must render "+
					"as no chip, never as the 已注销用户 placeholder", got)
			}
		})
	}
}

func TestFrozenCreatorIDs_SkipsUnknownAndDeduplicates(t *testing.T) {
	ids := []int{1, 2, 3, 4, 5}
	localMap := map[int]repository.GalgameLocalRow{
		1: {ID: 1, CreatorUserID: creatorID(42)},
		2: {ID: 2}, // NULL creator
		3: {ID: 3, CreatorUserID: creatorID(9)},
		4: {ID: 4, CreatorUserID: creatorID(42)}, // same author again
		// 5 absent: catalog-only work, no local row.
	}

	got := frozenCreatorIDs(ids, localMap)

	want := []int{42, 9}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v (card order, deduplicated)", got, want)
		}
	}
	for _, id := range got {
		if id == 0 {
			t.Fatal("user 0 was queued for hydration — Hydrate answers with a " +
				"已注销用户 placeholder for any id it is asked about, so an unknown " +
				"creator would render as a deleted account instead of no chip")
		}
	}
}
