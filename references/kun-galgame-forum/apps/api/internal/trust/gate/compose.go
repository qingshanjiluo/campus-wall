package gate

import (
	"strings"

	"kun-galgame-api/pkg/errors"
)

// ErrContentBlocked is the single-source 422-class response returned when the
// synchronous trust word-list gate DENIES a write (a banned term). Nothing is
// persisted. The message stays deliberately vague so the exact banned lexicon
// can't be probed. Wave 1's topic/reply helper delegates here so every write
// surface shares one copy of the string.
func ErrContentBlocked() *errors.AppError {
	return errors.New(errors.CodeBiz, "内容包含违禁词，无法发布", 422)
}

// ComposeText joins several raw text fragments into the single blob sent to the
// trust check + scan faces, skipping blank fragments (onboarding §5: send RAW
// text). Used by write paths whose moderatable content spans multiple fields
// (poll title + description + options, quiz question + explanation + content,
// resource note + links, …). Fragments are joined with a newline; the exact
// separator is immaterial to the word-list matcher.
func ComposeText(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
