package service

// The tag vocabulary index, precomputed and cached.
//
// The browse lane is upstream-paged, but the two rules this product applies to
// the vocabulary cannot be expressed upstream: the lane filters `tier` by
// EQUALITY (so "not hidden" is unaskable) and carries no filter on `sexual` at
// all. Both drops therefore happened AFTER pagination — a page of 100 came back
// with however many rows survived, above a `total` that still counted the ones
// removed.
//
// That was survivable while the do-not-display tier held nine words. The
// classification wave grew it to 62 (platform words and site-wide truisms —
// 游戏 / PC / Galgame / ADV / 黄油) and the adult vocabulary from 371 to 458, so a
// SFW reader now loses roughly a third of every page and the pager promises
// several pages that turn out to be empty.
//
// The whole vocabulary is ~1,700 rows, so it is walked once, filtered, and cut
// into pages here: full pages, a `total` that matches what the caller can
// actually reach, and one upstream walk per TTL instead of one per page view.
// The walk it replaces was not free either — the keyset lane has no offset, so
// reaching kungal page N re-walked N pages of it.

import (
	"context"
	"net/url"
	"sort"
	"time"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

const (
	// tagIndexTTL is how stale the vocabulary may be. It moves when a term is
	// reclassified or a work gains a tag, neither of which is urgent on a
	// browse index.
	tagIndexTTL = 10 * time.Minute
	// tagIndexPageCap bounds the walk at 100 rows a page — 4,000 canonical
	// tags against today's ~1,700. A backstop, not a working limit.
	tagIndexPageCap = 40
)

// indexedTag is one browse row plus the flag the response does not carry: the
// row's own category already encodes it, but the gate reads it directly rather
// than string-matching a rendered value.
type indexedTag struct {
	item   dto.TagListItem
	sexual bool
}

// indexRows returns the cached vocabulary, rebuilding when it is missing or
// stale.
func (s *TagService) indexRows(ctx context.Context) ([]indexedTag, *errors.AppError) {
	return s.index.get(ctx, tagIndexTTL, s.buildIndex)
}

// buildIndex walks the whole tag browse lane and keeps the listable rows.
func (s *TagService) buildIndex(ctx context.Context) ([]indexedTag, *errors.AppError) {
	// has_works=1 drops the empty vocabulary upstream — a browse page of "+ 0"
	// chips is a list of dead ends, and the filter is the same predicate
	// upstream counts with, so "count > 0" and "row present" cannot drift.
	base := client.OpenPopulation(url.Values{"has_works": {"1"}, "limit": {"100"}})
	rows := make([]indexedTag, 0, 2000)
	cursor := ""
	for page := 0; page < tagIndexPageCap; page++ {
		q := url.Values{}
		for k, v := range base {
			q[k] = v
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		res, appErr := s.galgameClient.CatalogTaxonomyList(ctx, "tags", q)
		if appErr != nil {
			return nil, appErr
		}
		for _, t := range res.Items {
			// Hidden-tier terms leave the vocabulary entirely: upstream parks
			// junk there, and they belong on no list, in no search and in no
			// picker — only on their own page, reached by a direct link.
			if t.Tier == client.TagTierHidden {
				continue
			}
			rows = append(rows, indexedTag{
				item: dto.TagListItem{
					ID: int(t.ID), Name: t.Label(),
					Category: tagCategory(t.Kind, t.Sexual), GalgameCount: t.WorkCount,
				},
				sexual: t.Sexual,
			})
		}
		if res.NextCursor == nil || *res.NextCursor == "" {
			break
		}
		cursor = *res.NextCursor
	}

	// Most-used first. The lane's own order is the upstream import order, which
	// says nothing to a reader; a 资料库 is browsed for the terms that actually
	// carry games. This ordering only became usable in this wave — before it,
	// the head of the list was 游戏 / PC / Galgame, truisms that sat on top
	// precisely because they are on everything. Name breaks ties so paging is
	// stable across rebuilds.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].item.GalgameCount != rows[j].item.GalgameCount {
			return rows[i].item.GalgameCount > rows[j].item.GalgameCount
		}
		return rows[i].item.Name < rows[j].item.Name
	})
	return rows, nil
}

// sexualByID answers the SFW gate for a handful of tag ids off the index.
//
// The entity-search hit shape carries no `sexual` flag, so the search gate used
// to resolve it one detail call per hit. The index already holds the answer for
// every tag that has works; ids it does not cover (the ~100 empty terms) are
// reported missing so the caller can fall back rather than call them safe.
func (s *TagService) sexualByID(ctx context.Context, ids []int) (sexual map[int]bool, missing []int) {
	sexual = make(map[int]bool, len(ids))
	rows, appErr := s.indexRows(ctx)
	if appErr != nil {
		return sexual, ids
	}
	want := make(map[int]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	seen := make(map[int]bool, len(ids))
	for _, r := range rows {
		if want[r.item.ID] {
			sexual[r.item.ID] = r.sexual
			seen[r.item.ID] = true
		}
	}
	for _, id := range ids {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	return sexual, missing
}
