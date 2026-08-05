package service

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/dto"
	"kun-galgame-api/pkg/errors"
)

// upcomingMonthCap bounds how many months forward the "未发售" aggregation
// fans out (current month inclusive) — a backstop against a stray far-future
// max_month. Real announced dates rarely exceed ~1–2 years out.
const upcomingMonthCap = 24

// upcomingFetchConcurrency caps concurrent calendar requests during the 未发售
// fan-out.
const upcomingFetchConcurrency = 8

// calendarPageLimit is how many rows a bucket page pulls. The month view
// renders the whole month at once, and 100 is the face's ceiling.
const calendarPageLimit = 100

// CalendarService serves the release-calendar buckets off the catalog calendar
// face and enriches each entry with forum-local data (view/like/platform +
// IsOnForum) via GalgameEnricher — the same overlay the entity detail pages
// use, so calendar cards match them.
//
// Population is the WHOLE catalog (refs/proj/126 P1), not just the works kungal
// has ingested, so most upcoming releases are rows with no local counterpart.
// They render as "未在论坛发布" claim cards whose link is name-based, which is
// why they need no kungal id — see CatalogItemToNextMoeItem.
type CalendarService struct {
	galgameClient *client.GalgameClient
	enricher      *GalgameEnricher
}

func NewCalendarService(galgameClient *client.GalgameClient, enricher *GalgameEnricher) *CalendarService {
	return &CalendarService{galgameClient: galgameClient, enricher: enricher}
}

// bucketQuery builds the shared population gates for every bucket.
//
// olang is deliberately left at the face's default (ja + the zh family): the
// catalog's galgame population is the full 226k cross-source registry, and a
// month view flooded with western VNDB entries is not the product. A lane that
// needs the whole set can pass olang=all.
func bucketQuery(isSFW bool) url.Values {
	q := url.Values{
		"limit":   {strconv.Itoa(calendarPageLimit)},
		"include": {CatalogCardInclude},
	}
	client.ApplyWorksGate(q, isSFW)
	return q
}

// fetchMonthRaw fetches one month bucket. An empty `month` lets the face
// default to the current JST month and echo back which one it served.
func (s *CalendarService) fetchMonthRaw(
	ctx context.Context,
	month string,
	isSFW bool,
) (*client.CatalogWorksPage, *errors.AppError) {
	q := bucketQuery(isSFW)
	if month != "" {
		q.Set("month", month)
	}
	return s.galgameClient.CatalogCalendar(ctx, "", q)
}

// GetMonth returns one ISO month's release list.
func (s *CalendarService) GetMonth(
	ctx context.Context,
	rawQuery url.Values,
	isSFW bool,
) (*dto.CalendarMonthPage, *errors.AppError) {
	page, appErr := s.fetchMonthRaw(ctx, rawQuery.Get("month"), isSFW)
	if appErr != nil {
		return nil, appErr
	}

	return &dto.CalendarMonthPage{
		Month: page.Month,
		Today: page.Meta.Today,
		Items: s.enricher.ToCards(ctx, catalogItemsToNextMoe(page.Items)),
		Meta: dto.CalendarMeta{
			// prev/next month are pure arithmetic on the served month; the face
			// supplies the has_prev/has_next flags that say whether stepping
			// there actually lands on data.
			PrevMonth: shiftMonth(page.Month, -1),
			NextMonth: shiftMonth(page.Month, +1),
			HasPrev:   derefBool(page.Meta.HasPrev),
			HasNext:   derefBool(page.Meta.HasNext),
			MinMonth:  page.Meta.MinMonth,
			MaxMonth:  page.Meta.MaxMonth,
			Count:     int(page.Count),
		},
	}, nil
}

// GetPending returns the "year known, month undecided" bucket for a year
// (empty `year` → current JST year, server-side).
func (s *CalendarService) GetPending(
	ctx context.Context,
	rawQuery url.Values,
	isSFW bool,
) (*dto.CalendarPendingPage, *errors.AppError) {
	q := bucketQuery(isSFW)
	if year := rawQuery.Get("year"); year != "" {
		q.Set("year", year)
	}
	page, appErr := s.galgameClient.CatalogCalendar(ctx, "/pending", q)
	if appErr != nil {
		return nil, appErr
	}
	return &dto.CalendarPendingPage{
		Year:  page.Year,
		Items: s.enricher.ToCards(ctx, catalogItemsToNextMoe(page.Items)),
		Count: int(page.Count),
	}, nil
}

// GetTBA returns the global "release date to be announced" bucket: works with
// an announced release that carries no year at all. A work with NO release row
// is "unknown", not "to be announced", and deliberately appears in no bucket.
func (s *CalendarService) GetTBA(
	ctx context.Context,
	rawQuery url.Values,
	isSFW bool,
) (*dto.CalendarTBAPage, *errors.AppError) {
	page, appErr := s.galgameClient.CatalogCalendar(ctx, "/tba", bucketQuery(isSFW))
	if appErr != nil {
		return nil, appErr
	}
	return &dto.CalendarTBAPage{
		Items: s.enricher.ToCards(ctx, catalogItemsToNextMoe(page.Items)),
		Count: int(page.Count),
	}, nil
}

// GetUpcoming aggregates the not-yet-released schedule: every dated entry
// (day/month precision) with release_date >= today, from the current month up
// to the data's max month, grouped by month. The current month is fetched first
// (it carries today + max_month); the remaining months fan out in parallel.
// Enrichment runs once over the union (one DB/OAuth batch) and the cards are
// then regrouped by month.
func (s *CalendarService) GetUpcoming(
	ctx context.Context,
	isSFW bool,
) (*dto.CalendarUpcomingPage, *errors.AppError) {
	base, appErr := s.fetchMonthRaw(ctx, "", isSFW)
	if appErr != nil {
		return nil, appErr
	}
	today := base.Meta.Today
	months := monthRange(base.Month, base.Meta.MaxMonth, upcomingMonthCap)

	// rawByMonth[0] is the already-fetched current month; the rest fan out.
	rawByMonth := make([][]client.CatalogWorkListItem, len(months))
	rawByMonth[0] = base.Items
	if len(months) > 1 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, upcomingFetchConcurrency)
		for i := 1; i < len(months); i++ {
			wg.Add(1)
			go func(idx int, m string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				// A single month failing must not sink the whole view — skip it.
				if resp, err := s.fetchMonthRaw(ctx, m, isSFW); err == nil {
					rawByMonth[idx] = resp.Items
				}
			}(i, months[i])
		}
		wg.Wait()
	}

	// Flatten in month order, keeping only the not-yet-released entries.
	// Lexicographic compare is valid on the partial-ISO date: a month-precision
	// "2026-08" sorts before every day in that month, which is the behaviour we
	// want (a month-precision entry in the current month stays visible).
	var flat []dto.NextMoeGalgameItem
	for _, items := range rawByMonth {
		for i := range items {
			it := &items[i]
			if !client.CatalogItemRenderable(it) {
				continue
			}
			if it.ReleaseDate != nil && *it.ReleaseDate >= monthOf(today) {
				flat = append(flat, client.CatalogItemToNextMoeItem(it))
			}
		}
	}

	// Enrich once, then regroup by the card's own month (release_date[:7]).
	cards := s.enricher.ToCards(ctx, flat)
	byMonth := make(map[string][]dto.GalgameCard, len(months))
	for _, c := range cards {
		if c.ReleaseDate == nil || len(*c.ReleaseDate) < 7 {
			continue
		}
		key := (*c.ReleaseDate)[:7]
		byMonth[key] = append(byMonth[key], c)
	}

	out := make([]dto.CalendarUpcomingMonth, 0, len(months))
	total := 0
	for _, m := range months {
		if g := byMonth[m]; len(g) > 0 {
			out = append(out, dto.CalendarUpcomingMonth{Month: m, Items: g})
			total += len(g)
		}
	}
	return &dto.CalendarUpcomingPage{Today: today, Months: out, Count: total}, nil
}

// monthOf truncates a YYYY-MM-DD date to its month, the comparison floor for
// "not yet released": an entry dated only to the month must not be dropped just
// because the month has already begun.
func monthOf(today string) string {
	if len(today) < 7 {
		return today
	}
	return today[:7]
}

func derefBool(p *bool) bool {
	return p != nil && *p
}

// shiftMonth steps a "YYYY-MM" string by n months.
func shiftMonth(ym string, n int) string {
	y, m := parseYM(ym)
	if y == 0 {
		return ""
	}
	total := y*12 + (m - 1) + n
	return formatYM(total/12, total%12+1)
}

// monthRange returns the inclusive list of "YYYY-MM" months from start to end,
// capped at `cap` entries. If end precedes start (no future data), only the
// start month is returned.
func monthRange(start, end string, cap int) []string {
	sy, sm := parseYM(start)
	ey, em := parseYM(end)
	if ey < sy || (ey == sy && em < sm) {
		return []string{formatYM(sy, sm)}
	}
	out := make([]string, 0, cap)
	y, m := sy, sm
	for len(out) < cap {
		out = append(out, formatYM(y, m))
		if y == ey && m == em {
			break
		}
		if m++; m > 12 {
			m = 1
			y++
		}
	}
	return out
}

func parseYM(s string) (int, int) {
	if len(s) < 7 {
		return 0, 1
	}
	y, _ := strconv.Atoi(s[:4])
	m, _ := strconv.Atoi(s[5:7])
	if m < 1 || m > 12 {
		m = 1
	}
	return y, m
}

func formatYM(y, m int) string {
	return fmt.Sprintf("%04d-%02d", y, m)
}
