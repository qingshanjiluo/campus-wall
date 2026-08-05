package markdown

import (
	"strings"
	"testing"
)

// TestRenderInline_Allows verifies the inline tier renders the "enough" set:
// emphasis, code, links, strikethrough, images, and line breaks.
func TestRenderInline_Allows(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		expect string // substring that MUST be present
	}{
		{"bold", "**hi**", "<strong>hi</strong>"},
		{"italic", "*hi*", "<em>hi</em>"},
		{"code", "`x := 1`", "<code>x := 1</code>"},
		{"strikethrough", "~~no~~", "<del>no</del>"},
		{"image", "![cat](https://image.kungal.com/c.png)", `<img src="https://image.kungal.com/c.png"`},
		{"link", "[k](https://kungal.org)", `href="https://kungal.org"`},
		{"hardwrap", "a\nb", "<br"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderInline(c.src)
			if !strings.Contains(got, c.expect) {
				t.Errorf("RenderInline(%q) = %q, want substring %q", c.src, got, c.expect)
			}
		})
	}
}

// TestRenderInline_BlockSyntaxStaysLiteral verifies block markdown is NOT
// promoted — the only block parser is the paragraph parser, so headings/lists/
// quotes/fences degrade to their literal source text inside a <p>.
func TestRenderInline_BlockSyntaxStaysLiteral(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		banned  string // tag that must NOT appear
		literal string // source text that MUST survive as text
	}{
		{"heading", "# Title", "<h1", "# Title"},
		{"list", "- item", "<li", "- item"},
		{"quote", "> quote", "<blockquote", "&gt; quote"},
		{"table", "| a | b |\n|---|---|", "<table", "| a | b |"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderInline(c.src)
			if strings.Contains(got, c.banned) {
				t.Errorf("RenderInline(%q) = %q, must NOT contain %q", c.src, got, c.banned)
			}
			if !strings.Contains(got, c.literal) {
				t.Errorf("RenderInline(%q) = %q, want literal %q", c.src, got, c.literal)
			}
		})
	}
}

// TestRenderInline_Sanitizes verifies the policy strips every XSS vector — raw
// HTML, script, event handlers, and dangerous URL schemes.
func TestRenderInline_Sanitizes(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		banned string
	}{
		{"script", "<script>alert(1)</script>", "<script"},
		{"img onerror", `<img src=x onerror="alert(1)">`, "onerror"},
		{"js link", "[x](javascript:alert(1))", "javascript:"},
		{"js image", "![x](javascript:alert(1))", "javascript:"},
		{"data uri", "![x](data:text/html,<script>alert(1)</script>)", "data:"},
		{"raw html div", "<div onclick=alert(1)>x</div>", "onclick"},
		{"iframe", `<iframe src="https://evil.test"></iframe>`, "<iframe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RenderInline(c.src)
			if strings.Contains(got, c.banned) {
				t.Errorf("RenderInline(%q) = %q, must NOT contain %q", c.src, got, c.banned)
			}
		})
	}
}

// TestRenderInline_ImageHostAllowlist verifies <img src> is restricted to our
// own image hosts — the core anti-tracking control for private-message images.
func TestRenderInline_ImageHostAllowlist(t *testing.T) {
	t.Run("allowed hosts keep src", func(t *testing.T) {
		for _, src := range []string{
			"https://image.kungal.iloveren.link/message/a.webp", // prod CDN
			"https://image.kungal.com/topic/a.webp",
			"https://sticker.kungal.com/stickers/s.webp",
		} {
			got := RenderInline("![x](" + src + ")")
			if !strings.Contains(got, `src="`+src+`"`) {
				t.Errorf("RenderInline image %q = %q, want src kept", src, got)
			}
		}
	})

	t.Run("foreign/look-alike/insecure hosts lose src", func(t *testing.T) {
		cases := []struct {
			name   string
			src    string
			banned string // must NOT survive in output
		}{
			{"external", "https://evil.test/track.png", "evil.test"},
			{"lookalike suffix", "https://image.kungal.com.evil.test/p.png", "evil.test"},
			{"http downgrade", "http://image.kungal.com/a.png", "image.kungal.com"},
			{"protocol relative", "//image.kungal.com/a.png", "image.kungal.com"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				got := RenderInline("![x](" + c.src + ")")
				if strings.Contains(got, c.banned) {
					t.Errorf("RenderInline image %q = %q, must NOT contain %q (src should be stripped)", c.src, got, c.banned)
				}
			})
		}
	})
}

// TestRenderInline_Empty keeps the empty-string fast path honest.
func TestRenderInline_Empty(t *testing.T) {
	if got := RenderInline(""); got != "" {
		t.Errorf("RenderInline(\"\") = %q, want empty", got)
	}
}
