package markdown

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/util"
)

// mentionIDRe extracts the user id from a stored mention token
// [@name](kungal-user:<id>). It runs against the raw markdown SOURCE (pre-render),
// so it sees the token before the render transform rewrites it to a link.
var mentionIDRe = regexp.MustCompile(`kungal-user:(\d+)`)

// mentionLinkRe matches a rendered mention link by its data-uid (the only <a>
// that carries one). Keyed on data-uid rather than a fixed attribute order so it
// survives any attribute reshuffling by the sanitizer. Groups: 1 = up to & incl.
// `data-uid="`, 2 = id, 3 = the rest of the opening tag, 4 = `</a>`; the link
// text between them is consumed but not captured (it's being replaced).
var mentionLinkRe = regexp.MustCompile(`(<a [^>]*\bdata-uid=")(\d+)("[^>]*>)@?[^<]*(</a>)`)

// ExtractMentionIDs returns the unique, positive user ids @mentioned in raw
// markdown `content`, in first-seen order. Used to fan out mention notifications
// and to batch-resolve current display names at render time.
func ExtractMentionIDs(content string) []int {
	matches := mentionIDRe.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	seen := make(map[int]bool, len(matches))
	ids := make([]int, 0, len(matches))
	for _, m := range matches {
		id, err := strconv.Atoi(m[1])
		if err != nil || id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// Reference tokens + the migration "> 回复 " header — stripped for plain-text
// notification previews (we want "the words said after the @", not the markup).
var (
	mentionTokenRe = regexp.MustCompile(`\[@[^\]]*\]\(kungal-user:\d+\)`)
	quoteTokenRe   = regexp.MustCompile(`\[#[^\]]*\]\(kungal-reply:\d+\)`)
	replyHeaderRe  = regexp.MustCompile(`(?m)^\s*>\s*回复\s*`)
)

// StripReferenceTokens removes @mention / #quote link tokens and the migration
// reply-header prefix from raw markdown, collapsing whitespace into single
// spaces — yielding the human-written text for a notification preview. Returns
// "" when the body is nothing but references, so callers can fall back to the
// generic "mentioned you" line.
func StripReferenceTokens(content string) string {
	s := mentionTokenRe.ReplaceAllString(content, "")
	s = quoteTokenRe.ReplaceAllString(s, "")
	s = replyHeaderRe.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// ResolveMentionNames rewrites the display text of rendered @mention links to the
// CURRENT display name from `names` (user id → name), so a renamed user shows
// their new name without the stored post ever changing. A mention whose id isn't
// in the map keeps its write-time snapshot text (e.g. a since-deleted user); the
// link target (id / href) is never touched. Call on the HTML from Render, with
// `names` batch-resolved from OAuth (/users/batch) — the topic/reply mappers
// already fetch the same batch for post authors, so mentions ride along for free.
func ResolveMentionNames(html string, names map[int]string) string {
	if len(names) == 0 {
		return html
	}
	return mentionLinkRe.ReplaceAllStringFunc(html, func(m string) string {
		sub := mentionLinkRe.FindStringSubmatch(m)
		id, _ := strconv.Atoi(sub[2])
		name, ok := names[id]
		if !ok || name == "" {
			return m // no resolution — keep the snapshot text
		}
		return sub[1] + sub[2] + sub[3] + "@" + string(util.EscapeHTML([]byte(name))) + sub[4]
	})
}
