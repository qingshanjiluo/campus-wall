package dto

// Release-calendar response shapes. These wrap the galgame calendar endpoints
// (GET /galgame/calendar[/pending|/tba]) after local enrichment, re-keying the
// galgame's snake_case meta to the camelCase the frontend consumes elsewhere.
// See docs/galgame_wiki/01-galgame.md §Galgame 发售月历.

// CalendarMeta is the month-navigation envelope. HasPrev/HasNext are
// data-boundary clamps (no day/month-precision data before MinMonth or after
// MaxMonth, per the current content_limit), so the FE disables paging at the
// edges instead of walking into empty months forever.
type CalendarMeta struct {
	PrevMonth string `json:"prev_month"`
	NextMonth string `json:"next_month"`
	HasPrev   bool   `json:"has_prev"`
	HasNext   bool   `json:"has_next"`
	MinMonth  string `json:"min_month"`
	MaxMonth  string `json:"max_month"`
	Count     int    `json:"count"`
}

// CalendarMonthPage is one ISO month's release list (day + month precision),
// already sorted by the galgame (date asc; within a day, exact-day entries before
// the "日未定" month-precision tail). Today is JST, for the 今日 marker.
type CalendarMonthPage struct {
	Month string        `json:"month"`
	Today string        `json:"today"`
	Items []GalgameCard `json:"items"`
	Meta  CalendarMeta  `json:"meta"`
}

// CalendarPendingPage is the "year known, month undecided" bucket
// (release_precision='year') for a given year.
type CalendarPendingPage struct {
	Year  string        `json:"year"`
	Items []GalgameCard `json:"items"`
	Count int           `json:"count"`
}

// CalendarTBAPage is the global "release date to be announced" bucket
// (release_precision='tba').
type CalendarTBAPage struct {
	Items []GalgameCard `json:"items"`
	Count int           `json:"count"`
}

// CalendarUpcomingMonth groups one month's not-yet-released entries.
type CalendarUpcomingMonth struct {
	Month string        `json:"month"`
	Items []GalgameCard `json:"items"`
}

// CalendarUpcomingPage is the consolidated "未发售" view: every dated entry
// (day/month precision) with release_date >= today, aggregated forward from
// the current month and grouped by month so the reader sees the whole release
// schedule without paging. The year-only / TBA buckets keep their own tabs.
type CalendarUpcomingPage struct {
	Today  string                  `json:"today"`
	Months []CalendarUpcomingMonth `json:"months"`
	Count  int                     `json:"count"`
}
