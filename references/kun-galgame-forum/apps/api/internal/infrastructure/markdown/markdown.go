package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"kun-galgame-api/pkg/imageclient"

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var (
	spoilerRegex   = regexp.MustCompile(`\|\|(.*?)\|\|`)
	videoLinkRegex = regexp.MustCompile(`kv:<a href="(https?://[^\s]+?\.(mp4))">[^<]+</a>`)
	codeBlockRegex = regexp.MustCompile(`(?s)<pre><code class="language-(\w+)"`)
	// Mention / quote tokens. The editor serializes a mention as the markdown
	// link [@name](kungal-user:<id>) and a quote as [#<floor>](kungal-reply:<id>);
	// goldmark renders both to a plain <a href="kungal-…:N">…</a> with the custom
	// scheme intact, which the transforms below rewrite (BEFORE sanitize) into the
	// safe mention link / quote span. Storing user.id (not the name) makes renames
	// render correctly — the link text is a write-time snapshot the mapper
	// re-resolves to the current display name.
	mentionRegex = regexp.MustCompile(`<a href="kungal-user:(\d+)"[^>]*>(.*?)</a>`)
	quoteRegex   = regexp.MustCompile(`<a href="kungal-reply:(\d+)"[^>]*>#?(\d+)</a>`)

	md         goldmark.Markdown
	mdHardWrap goldmark.Markdown
	sanitizer  *bluemonday.Policy
)

// ──────────────────────────────────────────
// Domain-independent content image references (/image/<hash>)
// ──────────────────────────────────────────
//
// Content (topic / reply / chat / comment markdown) stores image refs as the
// domain-independent token `/image/<hash>` instead of an absolute CDN URL, so a
// CDN/domain change is one config flip — not a rewrite of every historical row
// (image_service contract, docs/image_service/06-integration-guide.md). We
// resolve the token to the real CDN URL HERE, at the image-render step (i.e.
// BEFORE sanitization), so the sanitizer sees a normal https URL on an allowed
// host — relative URLs would otherwise be stripped (inline.go allow-list). The
// /image/:hash 302 route on the web origin is the fallback for anything not
// server-rendered (editor preview, RSS, raw API consumers).

// contentImageRefRe matches a /image/<hash> token: a 64-hex content hash with an
// optional _<variant> suffix. No extension — the resolver always emits .webp via
// imageclient (V1 outputs are always webp).
var contentImageRefRe = regexp.MustCompile(`^/image/([0-9a-f]{64})(?:_([a-z0-9]+))?$`)

// contentImageScanRe finds /image/<hash> tokens ANYWHERE in raw markdown (the
// non-anchored sibling of contentImageRefRe). The optional _variant is ignored —
// a topic cover is the base hash.
var contentImageScanRe = regexp.MustCompile(`/image/[0-9a-f]{64}`)

// ExtractContentImages returns up to `limit` distinct /image/<hash> tokens in
// first-seen (body) order from the markdown content — used to auto-pick a topic's
// covers from its body when the author set none.
func ExtractContentImages(content string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, limit)
	for _, tk := range contentImageScanRe.FindAllString(content, -1) {
		if _, dup := seen[tk]; dup {
			continue
		}
		seen[tk] = struct{}{}
		out = append(out, tk)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// contentImageCDNBase is the image_service public CDN prefix used to resolve
// /image/<hash> tokens. Set once at startup via SetContentImageCDNBase; empty =
// no resolution (tokens render verbatim, which the /image/:hash 302 then covers).
var contentImageCDNBase string

// SetContentImageCDNBase configures the CDN base for resolving /image/<hash>
// content refs. Call once at startup with cfg.NextMoeAPI.ImageCDNBase.
func SetContentImageCDNBase(base string) {
	contentImageCDNBase = strings.TrimRight(base, "/")
}

// contentSiteBase is the absolute web origin (e.g. https://www.kungal.com) used
// to render @mention links. UGCPolicy strips relative URLs, so the mention <a>
// needs an absolute href; data-uid carries the id for SPA nav and name
// re-resolution either way. Empty = relative href (stripped by the sanitizer,
// but the chip + data-uid still survive, so the mention stays usable).
var contentSiteBase string

// SetContentSiteBase configures the origin for @mention link hrefs. Call once at
// startup with the web origin.
func SetContentSiteBase(base string) {
	contentSiteBase = strings.TrimRight(base, "/")
}

// resolveContentImageRef turns a /image/<hash>[_variant] token into the absolute
// CDN URL via imageclient.MainURL/VariantURL (the contract path layout —
// {base}/<aa>/<bb>/<hash>.webp). Returns "" for anything that isn't a token
// (absolute/external/legacy URLs render unchanged).
func resolveContentImageRef(dest string) string {
	if contentImageCDNBase == "" {
		return ""
	}
	m := contentImageRefRe.FindStringSubmatch(dest)
	if m == nil {
		return ""
	}
	hash, variant := m[1], m[2]
	if variant != "" {
		return imageclient.VariantURL(contentImageCDNBase, hash, variant, "webp")
	}
	return imageclient.MainURL(contentImageCDNBase, hash, "webp")
}

// TocLink is one entry in a table of contents tree. The JSON shape matches
// DocTocLink on the frontend.
type TocLink struct {
	ID       string    `json:"id"`
	Text     string    `json:"text"`
	Depth    int       `json:"depth"`
	Children []TocLink `json:"children,omitempty"`
}

func init() {
	md = newGoldmark(false)
	// Hard-wrap variant: every newline becomes a <br>. Used for the galgame
	// resource note, which was a plain-text <textarea> before it became Markdown,
	// so single newlines in existing notes are real line breaks and must survive
	// (CommonMark would otherwise fold a lone newline into a space). See
	// RenderHardWrap.
	mdHardWrap = newGoldmark(true)

	sanitizer = newSanitizePolicy()
}

// newGoldmark builds a goldmark instance with the project's shared extension /
// parser config. hardWraps adds html.WithHardWraps() so a single newline renders
// as <br> instead of a space.
func newGoldmark(hardWraps bool) goldmark.Markdown {
	// No server-side syntax highlighting. goldmark-highlighting (Chroma, style
	// "monokai") stamps a hard-coded dark inline background onto <pre> — so code
	// blocks render dark in BOTH light/dark mode — and it rewrites the markup to
	// `<pre class="chroma">`, which bypasses the codeBlockRegex wrapper below.
	// Emitting plain `<pre><code class="language-x">` lets every fence flow
	// through the .kun-code-container wrapper and be themed by the shared
	// prose.css (project color system), matching moyu / galgame.
	rendererOpts := []renderer.Option{html.WithUnsafe()}
	if hardWraps {
		rendererOpts = append(rendererOpts, html.WithHardWraps())
	}
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			mathjax.MathJax,
			&h1ToH2Extension{},
			&lazyImageExtension{},
		),
		// Heading ids: enable goldmark's auto-heading-id transformer; the
		// actual slug algorithm is injected per-call via parser.WithIDs so
		// Chinese/Japanese headings keep their Unicode characters instead
		// of being stripped to empty strings.
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			// Attach each content image's dims + ThumbHash (one batched lookup
			// per document) so the rendered <img> reserves its aspect ratio and
			// blurs up — see contentImageMetaTransformer.
			parser.WithASTTransformers(
				util.Prioritized(&contentImageMetaTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(rendererOpts...),
	)
}

// newSanitizePolicy builds the allow-list applied to goldmark's HTML output.
// goldmark runs with html.WithUnsafe(), so raw user HTML passes through and the
// result is UNTRUSTED — this is the single server-side sanitization boundary
// (it replaces the old per-render client-side DOMPurify, which ran jsdom on
// every SSR render and leaked memory). The allow-list keeps everything the
// kungal pipeline emits — heading-id anchors, code language classes, math
// spans, lazy-image attrs, GFM tables/task-lists, and the post-render
// code/spoiler/table/video wrappers — while stripping <script>/<style>, event
// handlers, and unsafe URL schemes (javascript:, data:, …).
func newSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()

	// Classes carry no script and are required by code (language-*), math,
	// spoilers, code containers, and the prose styling.
	p.AllowAttrs("class").Globally()
	// Auto heading-id anchors (parser.WithAutoHeadingID).
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	// lazyImageRenderer attributes.
	p.AllowAttrs("loading", "decoding", "data-kun-lazy-image").OnElements("img")
	// Content-image metadata stamped by contentImageMetaTransformer: width /
	// height reserve the aspect ratio (no CLS), data-thumbhash drives the
	// frontend blur-up. Value-constrained (digits / base64) so raw user HTML
	// can't smuggle anything else through these attributes.
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("img")
	p.AllowAttrs("data-thumbhash").Matching(regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)).OnElements("img")
	// Markup added by the post-render transforms (the <video> src is regex-
	// constrained to https?://….mp4; the button carries no handler).
	p.AllowElements("video", "button")
	p.AllowAttrs("controls", "loop", "playsinline", "width", "src").OnElements("video")
	p.AllowAttrs("title").OnElements("button")
	// GFM task-list checkboxes.
	p.AllowElements("input")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	// Mention / quote tokens (post-render transforms): data-uid on the mention
	// <a>, data-reply-id/data-floor on the quote <span>. (class is already global;
	// <a>/<span> are UGCPolicy elements.)
	p.AllowAttrs("data-uid").OnElements("a")
	p.AllowAttrs("data-reply-id", "data-floor").OnElements("span")

	return p
}

// Render converts markdown to HTML with all custom transformations.
func Render(source string) string {
	html, _ := RenderWithTOC(source)
	return html
}

// RenderHardWrap is Render with hard line breaks (every newline → <br>). Use it
// for content that was plain text before becoming Markdown — currently the
// galgame resource note, where existing notes' single-newline line breaks must
// survive. Same extensions / transforms / sanitization as Render; no TOC.
func RenderHardWrap(source string) string {
	return renderWith(mdHardWrap, source)
}

// RenderWithTOC converts markdown to HTML and also returns a nested TOC tree
// built from the document's h2/h3 headings (h1 is promoted to h2 to match the
// h1→h2 render transform).
func RenderWithTOC(source string) (string, []TocLink) {
	src := []byte(source)
	reader := text.NewReader(src)
	ctx := parser.NewContext(parser.WithIDs(newUnicodeIDs()))
	root := md.Parser().Parse(reader, parser.WithContext(ctx))

	toc := buildTOCTree(collectHeadings(root, src), 3)

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, root); err != nil {
		return source, nil
	}

	return applyTransforms(buf.String()), toc
}

// renderWith parses + renders `source` with the given goldmark instance and runs
// the shared post-render transforms. Used by RenderHardWrap (and mirrors what
// RenderWithTOC does, minus the TOC).
func renderWith(m goldmark.Markdown, source string) string {
	src := []byte(source)
	reader := text.NewReader(src)
	ctx := parser.NewContext(parser.WithIDs(newUnicodeIDs()))
	root := m.Parser().Parse(reader, parser.WithContext(ctx))

	var buf bytes.Buffer
	if err := m.Renderer().Render(&buf, src, root); err != nil {
		return source
	}

	return applyTransforms(buf.String())
}

// applyTransforms runs the post-render HTML rewrites (code/table/spoiler/video
// wrappers) and the single server-side sanitization pass shared by every
// renderer entry point.
func applyTransforms(result string) string {
	// Code block wrapper:
	// <pre><code class="language-go"... → wrapped in div.kun-code-container
	result = codeBlockRegex.ReplaceAllStringFunc(result, func(match string) string {
		lang := codeBlockRegex.FindStringSubmatch(match)
		if len(lang) < 2 {
			return match
		}
		return `<div class="kun-code-container language-` + lang[1] + `">` +
			`<div class="kun-code-header">` +
			`<span class="lang">` + lang[1] + `</span>` +
			`<button class="copy" title="Copy code"></button>` +
			`</div>` +
			`<pre><code class="language-` + lang[1] + `"`
	})
	// No-language fenced / indented blocks render as a bare `<pre><code>` (no
	// language class), which the wrapper above skips — but the unconditional
	// `</code></pre></div>` close below STILL fires, leaving a stray `</div>`.
	// The browser then uses it to close an ancestor early, so the parsed SSR DOM
	// diverges from the raw HTML the client renders → cascading hydration
	// mismatches through the rest of the post. Wrap these too, so every close
	// has a matching open. (Language blocks are now `<pre><code class="…"` and
	// no longer match the bare `<pre><code>` literal.)
	result = strings.ReplaceAll(result, "<pre><code>",
		`<div class="kun-code-container">`+
			`<div class="kun-code-header">`+
			`<span class="lang"></span>`+
			`<button class="copy" title="Copy code"></button>`+
			`</div>`+
			`<pre><code>`)
	result = strings.ReplaceAll(result, "</code></pre>", "</code></pre></div>")

	// Table wrapper
	result = strings.ReplaceAll(result, "<table>", `<div class="kun-table-container"><table>`)
	result = strings.ReplaceAll(result, "</table>", `</table></div>`)

	// Spoiler: ||text|| → <span class="kun-spoiler ...">text</span>
	result = spoilerRegex.ReplaceAllString(result,
		`<span class="kun-spoiler text-transparent kun-spoiler-hidden">$1</span>`)

	// Video: kv:<a href="url.mp4">...</a> → <video>
	result = videoLinkRegex.ReplaceAllString(result,
		`<video controls loop playsinline width="100%" src="$1"></video>`)

	// Mention: <a href="kungal-user:N">@name</a> → safe mention link. Absolute
	// href (relative is stripped by the sanitizer); data-uid drives SPA nav and
	// lets the mapper re-resolve the current display name from the id.
	result = mentionRegex.ReplaceAllString(result,
		`<a class="kun-mention" data-uid="$1" href="`+contentSiteBase+`/user/$1/info">$2</a>`)
	// Quote: <a href="kungal-reply:N">#F</a> → a quote span the frontend hydrates
	// into a card (lazy preview + jump). No href avoids the relative-URL strip;
	// data-reply-id + data-floor carry the target.
	result = quoteRegex.ReplaceAllString(result,
		`<span class="kun-quote" data-reply-id="$1" data-floor="$2">#$2</span>`)

	// Sanitize the full rendered HTML (incl. the transform-added wrappers and
	// any raw user HTML that html.WithUnsafe let through). Done once here,
	// server-side; the frontend now renders contentHtml directly without jsdom.
	return sanitizer.Sanitize(result)
}

// ──────────────────────────────────────────
// Heading IDs + TOC extraction
// ──────────────────────────────────────────

// unicodeIDs is a goldmark parser.IDs that keeps Unicode letters/digits
// (CJK friendly) instead of stripping them like goldmark's default ASCII-only
// generator. A fresh instance is used per Parse so dedupe state does not
// leak across documents.
type unicodeIDs struct {
	used map[string]int
	anon int
}

func newUnicodeIDs() *unicodeIDs {
	return &unicodeIDs{used: map[string]int{}}
}

func (u *unicodeIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := slugify(string(value))
	if base == "" {
		base = fmt.Sprintf("heading-%d", u.anon)
		u.anon++
	}
	id := base
	if n := u.used[base]; n > 0 {
		u.used[base] = n + 1
		id = fmt.Sprintf("%s-%d", base, n)
	} else {
		u.used[base] = 1
	}
	u.used[id] = 1
	return []byte(id)
}

func (u *unicodeIDs) Put(value []byte) {
	u.used[string(value)] = 1
}

// collectHeadings walks the AST and returns flat heading entries using the
// `id` attribute goldmark already stamped via unicodeIDs. h1 is promoted to
// depth 2 so TOC nesting aligns with the h1→h2 render transform.
func collectHeadings(root ast.Node, source []byte) []TocLink {
	if root == nil {
		return nil
	}
	var out []TocLink
	ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		depth := h.Level
		if depth == 1 {
			depth = 2
		}
		out = append(out, TocLink{
			ID:    headingID(h),
			Text:  headingText(h, source),
			Depth: depth,
		})
		return ast.WalkContinue, nil
	})
	return out
}

func headingID(h *ast.Heading) string {
	attr, found := h.Attribute([]byte("id"))
	if !found {
		return ""
	}
	if b, ok := attr.([]byte); ok {
		return string(b)
	}
	return ""
}

// headingText concatenates the inline text of a heading, dropping markdown
// formatting (e.g., **bold** → bold) so TOC labels match the rendered page.
func headingText(h *ast.Heading, source []byte) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		if t, ok := n.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
			return
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(h)
	return strings.TrimSpace(b.String())
}

// slugify builds a URL-friendly id that preserves Unicode letters/digits so
// Chinese and Japanese headings get meaningful ids instead of being stripped
// to empty strings like goldmark's default ASCII-only generator does.
func slugify(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			if r < 128 {
				b.WriteRune(unicode.ToLower(r))
			} else {
				b.WriteRune(r)
			}
			prevDash = false
		case unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		case unicode.IsSpace(r), r == '-', r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	runes := []rune(out)
	if len(runes) > 100 {
		out = string(runes[:100])
	}
	return out
}

// buildTOCTree converts a flat heading list into a nested tree. Headings
// outside [2, maxDepth] are skipped (h1 was promoted to depth 2 upstream).
func buildTOCTree(flat []TocLink, maxDepth int) []TocLink {
	var roots []TocLink
	// Stack holds pointers into parent `Children` slices (or into roots for
	// the top level) so we can append nested entries in place.
	type frame struct {
		depth int
		list  *[]TocLink
	}
	stack := []frame{{depth: 1, list: &roots}}

	for _, h := range flat {
		if h.Depth < 2 || h.Depth > maxDepth {
			continue
		}
		for len(stack) > 1 && h.Depth <= stack[len(stack)-1].depth {
			stack = stack[:len(stack)-1]
		}
		top := stack[len(stack)-1]
		*top.list = append(*top.list, h)
		// Push a frame pointing at the new entry's children slice so the
		// next deeper heading lands inside it.
		newEntry := &(*top.list)[len(*top.list)-1]
		stack = append(stack, frame{depth: h.Depth, list: &newEntry.Children})
	}
	return roots
}

// ToPlainText strips markdown syntax and returns plain text, truncated to maxLen runes.
func ToPlainText(source string, maxLen int) string {
	text := source
	text = regexp.MustCompile(`!\[.*?\]\(.*?\)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`\[([^\]]*)\]\(.*?\)`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile("[#*_~>`|]").ReplaceAllString(text, "")
	text = regexp.MustCompile(`\n{2,}`).ReplaceAllString(text, "\n")
	text = strings.TrimSpace(text)

	runes := []rune(text)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return text
}

// ──────────────────────────────────────────
// Extension: H1 → H2
// ──────────────────────────────────────────

type h1ToH2Extension struct{}

func (e *h1ToH2Extension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&h1ToH2Renderer{}, 100),
	))
}

type h1ToH2Renderer struct{}

func (r *h1ToH2Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHeading, r.renderHeading)
}

func (r *h1ToH2Renderer) renderHeading(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.Heading)
	level := n.Level
	if level == 1 {
		level = 2
	}
	tag := byte('0' + level)

	if entering {
		w.WriteString("<h")
		w.WriteByte(tag)
		for _, attr := range n.Attributes() {
			w.WriteByte(' ')
			w.Write(attr.Name)
			w.WriteString(`="`)
			w.Write(util.EscapeHTML(attr.Value.([]byte)))
			w.WriteByte('"')
		}
		w.WriteByte('>')
	} else {
		w.WriteString("</h")
		w.WriteByte(tag)
		w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

// ──────────────────────────────────────────
// Extension: Lazy Image
// ──────────────────────────────────────────

type lazyImageExtension struct{}

func (e *lazyImageExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&lazyImageRenderer{}, 100),
	))
}

type lazyImageRenderer struct{}

func (r *lazyImageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
}

func (r *lazyImageRenderer) renderImage(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)

	// Collect alt text from child text nodes
	var altBuf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			altBuf.Write(t.Value(source))
		}
	}

	// Resolve a domain-independent /image/<hash> token to the absolute CDN URL
	// before writing (and before sanitization). Non-tokens pass through as-is.
	dest := n.Destination
	if resolved := resolveContentImageRef(string(dest)); resolved != "" {
		dest = []byte(resolved)
	}

	w.WriteString(`<img src="`)
	w.Write(util.EscapeHTML(dest))
	w.WriteString(`" alt="`)
	w.Write(util.EscapeHTML(altBuf.Bytes()))
	w.WriteString(`"`)
	if n.Title != nil {
		w.WriteString(` title="`)
		w.Write(util.EscapeHTML(n.Title))
		w.WriteString(`"`)
	}
	// Dims + ThumbHash attached by contentImageMetaTransformer (absent when the
	// image_service meta lookup missed or is disabled — then it's a plain
	// lazy <img>, exactly as before).
	writeImageAttr(w, n, "width")
	writeImageAttr(w, n, "height")
	writeImageAttr(w, n, "data-thumbhash")
	w.WriteString(` loading="lazy" decoding="async" data-kun-lazy-image="true" />`)
	return ast.WalkSkipChildren, nil
}

// writeImageAttr writes ` name="value"` for an attribute the meta transformer
// stamped on the image node, escaping the value. No-op if the attr is absent.
func writeImageAttr(w util.BufWriter, n *ast.Image, name string) {
	v, ok := n.AttributeString(name)
	if !ok {
		return
	}
	b, ok := v.([]byte)
	if !ok || len(b) == 0 {
		return
	}
	w.WriteByte(' ')
	w.WriteString(name)
	w.WriteString(`="`)
	w.Write(util.EscapeHTML(b))
	w.WriteByte('"')
}

// ──────────────────────────────────────────
// Extension: content image metadata (dims + ThumbHash)
// ──────────────────────────────────────────
//
// Stamps each content image with its intrinsic width/height (so the rendered
// <img> reserves its aspect ratio — no layout shift as it loads) and its
// base64 ThumbHash (data-thumbhash, which the frontend KunContent renderer
// blurs up in place). Runs as a parse-time AST transformer so it can batch-
// resolve EVERY image in the document through a single image_service meta
// lookup; the MetaResolver caches forever, so warm renders touch no network.
// No-op when the resolver is unset (image_service disabled) — images then
// render as a plain lazy <img>, exactly as before.

// contentImageMetaResolve resolves a batch of content-image hashes to their
// dims + ThumbHash. Set once at startup via SetContentImageMetaResolver; nil =
// no enrichment.
var contentImageMetaResolve func(hashes []string) map[string]imageclient.ImageMeta

// SetContentImageMetaResolver wires the image-metadata resolver used to enrich
// content <img> tags. Call once at startup with the image client's
// MetaResolver.Resolve; leave unset to disable enrichment.
func SetContentImageMetaResolver(fn func(hashes []string) map[string]imageclient.ImageMeta) {
	contentImageMetaResolve = fn
}

// ResolveContentImageMeta returns dims + ThumbHash for a set of /image/<hash>
// content tokens (e.g. a topic's cover tokens), keyed by the INPUT token so the
// caller can look meta up by what it already holds. Best-effort: tokens the
// resolver doesn't know (or non-tokens) are simply absent; returns nil when no
// resolver is wired (image_service disabled) so the caller renders plain images.
// Reuses the same cached resolver as the content-image render path.
func ResolveContentImageMeta(tokens []string) map[string]imageclient.ImageMeta {
	resolve := contentImageMetaResolve
	if resolve == nil || len(tokens) == 0 {
		return nil
	}
	hashByToken := make(map[string]string, len(tokens))
	hashes := make([]string, 0, len(tokens))
	for _, tk := range tokens {
		if h := contentImageHash(tk); h != "" {
			hashByToken[tk] = h
			hashes = append(hashes, h)
		}
	}
	if len(hashes) == 0 {
		return nil
	}
	metas := resolve(hashes)
	out := make(map[string]imageclient.ImageMeta, len(hashByToken))
	for tk, h := range hashByToken {
		if m, ok := metas[h]; ok {
			out[tk] = m
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// contentImageHash extracts the base 64-hex content hash from a /image/<hash>
// token, ignoring any _variant suffix (every variant of a hash shares the main
// image's aspect ratio). Returns "" for non-token destinations.
func contentImageHash(dest string) string {
	m := contentImageRefRe.FindStringSubmatch(dest)
	if m == nil {
		return ""
	}
	return m[1]
}

type contentImageMetaTransformer struct{}

func (t *contentImageMetaTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	resolve := contentImageMetaResolve
	if resolve == nil {
		return
	}

	var imgs []*ast.Image
	var hashes []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if img, ok := n.(*ast.Image); ok {
			if h := contentImageHash(string(img.Destination)); h != "" {
				imgs = append(imgs, img)
				hashes = append(hashes, h)
			}
		}
		return ast.WalkContinue, nil
	})
	if len(imgs) == 0 {
		return
	}

	metas := resolve(hashes)
	for i, img := range imgs {
		m, ok := metas[hashes[i]]
		if !ok {
			continue
		}
		if m.Width > 0 {
			img.SetAttributeString("width", []byte(strconv.Itoa(m.Width)))
		}
		if m.Height > 0 {
			img.SetAttributeString("height", []byte(strconv.Itoa(m.Height)))
		}
		if m.Thumbhash != "" {
			img.SetAttributeString("data-thumbhash", []byte(m.Thumbhash))
		}
	}
}
