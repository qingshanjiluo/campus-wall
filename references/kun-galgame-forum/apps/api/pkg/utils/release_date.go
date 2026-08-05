package utils

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Release-date filter bound parsing, mirroring the galgame §17 protocol
// (docs/galgame_wiki/00-handbook-for-downstream.md §17). Accepts two
// string formats and resolves each to an inclusive DATE boundary that
// kungal's local `galgame.release_date` (PG `date`) column compares
// against:
//
//	"YYYY"     whole year  → lower = Jan 1,  upper = Dec 31
//	"YYYY-MM"  whole month → lower = 1st,    upper = last day (28-31)
//	""         omitted     → no bound (returns "", caller skips WHERE)
//
// Anything else (e.g. "24", "2024-3" missing zero-pad, "2024-13",
// "garbage") is rejected so a malformed filter surfaces as 400 rather
// than silently returning the whole table.
//
// Output is a "YYYY-MM-DD" string fed straight into a parameterised
// `release_date >= ?` / `<= ?` comparison (PG casts the literal to
// date). We intentionally return date strings, not time.Time — the
// column is date-typed and tz-free, so day granularity is exact and
// avoids any tz drift (§17.6).

var (
	reYear  = regexp.MustCompile(`^\d{4}$`)
	reMonth = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
)

// ParseReleaseLowerBound resolves the inclusive lower edge. Empty input
// → ("", nil) meaning "no lower bound".
func ParseReleaseLowerBound(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if reYear.MatchString(s) {
		return s + "-01-01", nil
	}
	if m := reMonth.FindStringSubmatch(s); m != nil {
		if err := validMonth(m[2]); err != nil {
			return "", err
		}
		return m[1] + "-" + m[2] + "-01", nil
	}
	return "", fmt.Errorf("非法的发售日期下限 %q（应为 YYYY 或 YYYY-MM）", s)
}

// ParseReleaseUpperBound resolves the inclusive upper edge — the LAST
// day of the year/month so the comparison includes the whole period.
// Empty input → ("", nil) meaning "no upper bound".
func ParseReleaseUpperBound(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if reYear.MatchString(s) {
		return s + "-12-31", nil
	}
	if m := reMonth.FindStringSubmatch(s); m != nil {
		if err := validMonth(m[2]); err != nil {
			return "", err
		}
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		// Day 0 of the NEXT month is the last day of THIS month — handles
		// 28/29/30/31 (incl. leap years) without a lookup table.
		last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
		return last.Format("2006-01-02"), nil
	}
	return "", fmt.Errorf("非法的发售日期上限 %q（应为 YYYY 或 YYYY-MM）", s)
}

func validMonth(mm string) error {
	month, _ := strconv.Atoi(mm)
	if month < 1 || month > 12 {
		return fmt.Errorf("非法的月份 %q（应为 01-12）", mm)
	}
	return nil
}

// ParseMonthSet parses the `released_months` query param (galgame §17.10):
// a comma-separated set of month numbers (1–12) AND-combined with the
// year range to keep only games released in those months, across all
// years in range ("历年三月发售"). Returns a deduped, ascending slice.
//
//	""        → nil   (no month filter)
//	"3,7,12"  → [3 7 12]
//	"12,3,3"  → [3 12]  (dedupe + sort)
//
// Any non-1–12 / non-numeric token → error (caller maps to 400), so a
// malformed set fails loudly rather than silently widening results.
func ParseMonthSet(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	seen := map[int]bool{}
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		m, err := strconv.Atoi(tok)
		if err != nil || m < 1 || m > 12 {
			return nil, fmt.Errorf("非法的月份 %q（应为 1-12 的逗号分隔列表）", tok)
		}
		seen[m] = true
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Ints(out)
	return out, nil
}
