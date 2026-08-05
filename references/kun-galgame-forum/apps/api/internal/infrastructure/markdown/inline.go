package markdown

import (
	"bytes"
	"os"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Message markdown — INLINE constructs plus images only. This is the "enough"
// tier for chat / PMs: inline emphasis, code spans, links, strikethrough, line
// breaks, and images — NOT topic-grade block markdown (headings, lists, quotes,
// code fences, tables). Kept entirely separate from the topic `md`/`sanitizer`
// so the two policies can't drift into each other.
var (
	inlineMd        = newInlineMarkdown()
	inlineSanitizer = newInlineSanitizePolicy()
)

// allowedImageHosts is the allow-list of hosts a message <img src> may point at.
// Messages are restricted to OUR OWN image origins — never arbitrary external
// URLs — to close the private-message tracking-pixel / deanonymization vector:
// an external src would make the *recipient's* browser fetch an
// attacker-controlled URL on open, leaking their IP / UA / online-time (and the
// file behind an external URL can be swapped after delivery). The chat uploader
// posts to /image/message and returns an image.kungal.iloveren.link URL (the
// image_service KUN_IMAGE_PUBLIC_BASE_URL), so legitimate images always land on
// one of these hosts. (Topics, by contrast, allow external images — a public,
// different-risk surface; messages are private 1-to-1 and locked down.)
var allowedImageHosts = resolveAllowedImageHosts()

// resolveAllowedImageHosts reads KUNGAL_MESSAGE_IMAGE_HOSTS (comma-separated)
// when set, else falls back to the known production CDN hosts. The env hook
// lets dev/staging (local minio, a different CDN) or a future domain migration
// change the allow-list without a code change — the host MUST match
// image_service's KUN_IMAGE_PUBLIC_BASE_URL or uploaded images get stripped.
func resolveAllowedImageHosts() []string {
	if v := strings.TrimSpace(os.Getenv("KUNGAL_MESSAGE_IMAGE_HOSTS")); v != "" {
		var hosts []string
		for p := range strings.SplitSeq(v, ",") {
			if h := strings.TrimSpace(p); h != "" {
				hosts = append(hosts, h)
			}
		}
		if len(hosts) > 0 {
			return hosts
		}
	}
	return []string{
		"image.kungal.iloveren.link", // current production image CDN (KUN_IMAGE_PUBLIC_BASE_URL)
		"image.kungal.com",           // canonical domain (avatars reference it; future migration)
		"sticker.kungal.com",         // stickers
	}
}

// messageImageSrcPattern matches ONLY https URLs whose host is exactly one of
// allowedImageHosts. The required trailing "/" after the host defeats
// look-alikes like https://image.kungal.com.evil.test/x (the char after the
// host would be "." not "/"). Protocol-relative ("//host"), http, relative, and
// data:/javascript: URLs all fail to match and get their src stripped.
var messageImageSrcPattern = buildImageSrcPattern(allowedImageHosts)

func buildImageSrcPattern(hosts []string) *regexp.Regexp {
	escaped := make([]string, len(hosts))
	for i, h := range hosts {
		escaped[i] = regexp.QuoteMeta(h)
	}
	return regexp.MustCompile(`^https://(` + strings.Join(escaped, "|") + `)/`)
}

// newInlineMarkdown registers ONLY the paragraph block parser, so block syntax
// is never recognized: "# hi", "- a", "> q", and ``` fences stay as literal
// text inside a paragraph instead of becoming headings/lists/quotes/code
// blocks. The default inline parsers still run (emphasis, code span, link,
// image, autolink). Raw HTML is escaped — no html.WithUnsafe() here.
func newInlineMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParser(parser.NewParser(
			parser.WithBlockParsers(
				util.Prioritized(parser.NewParagraphParser(), 1000),
			),
			parser.WithInlineParsers(parser.DefaultInlineParsers()...),
			parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		)),
		goldmark.WithExtensions(
			extension.Strikethrough, // ~~del~~
			extension.Linkify,       // bare URLs become links
			&lazyImageExtension{},   // reuse topic lazy-image (loading/decoding attrs)
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // single newline → <br>, chat-style
		),
	)
}

// newInlineSanitizePolicy is a strict allow-list: inline formatting, links, and
// images only — no block elements, no raw HTML, no scripts/handlers. URLs are
// restricted to safe schemes (blocks javascript:, data:, …).
func newInlineSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "del", "code", "a", "img")
	p.AllowAttrs("href").OnElements("a")
	// img src is allow-listed to our own image hosts (see messageImageSrcPattern);
	// a non-matching src is stripped, neutralizing external/tracking images. The
	// remaining img attrs are non-URL (alt/title) or the lazy-load markers.
	p.AllowAttrs("src").Matching(messageImageSrcPattern).OnElements("img")
	p.AllowAttrs("alt", "title", "loading", "decoding", "data-kun-lazy-image").OnElements("img")
	// Links may point anywhere (a click is user-initiated, unlike an auto-loaded
	// image) — keep them safe with standard schemes + nofollow + target=_blank.
	p.AllowStandardURLs()
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}

// RenderInline renders message markdown to sanitized HTML — inline formatting
// plus images only. Output is safe to render with v-html.
func RenderInline(source string) string {
	if source == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := inlineMd.Convert([]byte(source), &buf); err != nil {
		return inlineSanitizer.Sanitize(source)
	}
	return inlineSanitizer.Sanitize(buf.String())
}
